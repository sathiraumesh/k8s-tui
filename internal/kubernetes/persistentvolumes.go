package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubectl/pkg/describe"
)

// PersistentVolume is one persistent volume (PV): a piece of storage of
// the cluster that a claim (PVC) of a namespace can be bound to. It has
// what `kubectl get persistentvolumes -o wide` shows of it.
type PersistentVolume struct {
	Name string

	Capacity      string // e.g. "10Gi"
	AccessModes   string // e.g. "RWO", "RWO,ROX"
	ReclaimPolicy string // what happens to it when its claim is deleted: Retain, Delete (or Recycle)
	Status        string // Available, Bound, Released, Failed or Pending
	Claim         string // the claim bound to it, NAMESPACE/NAME, or ""
	StorageClass  string // "" for none
	Reason        string // why it failed, usually ""
	VolumeMode    string // Filesystem or Block

	CreatedAt time.Time
}

// ListPersistentVolumes returns the persistent volumes, sorted by name, like
// `kubectl get persistentvolumes`. They aren't in a namespace.
func (c *Client) ListPersistentVolumes(ctx context.Context) ([]PersistentVolume, error) {
	list, err := c.clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	volumes := make([]PersistentVolume, 0, len(list.Items))
	for _, pv := range list.Items {
		volumes = append(volumes, toPersistentVolume(pv))
	}
	sort.Slice(volumes, func(i, j int) bool { return volumes[i].Name < volumes[j].Name })
	return volumes, nil
}

// toPersistentVolume converts client-go's corev1.PersistentVolume into
// ours, with its columns as kubectl writes them.
func toPersistentVolume(pv corev1.PersistentVolume) PersistentVolume {
	v := PersistentVolume{
		Name:          pv.Name,
		AccessModes:   accessModes(pv.Spec.AccessModes),
		ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy),
		Status:        string(pv.Status.Phase),
		StorageClass:  pv.Spec.StorageClassName,
		Reason:        pv.Status.Reason,
		VolumeMode:    string(corev1.PersistentVolumeFilesystem),
		CreatedAt:     pv.CreationTimestamp.Time,
	}
	if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		v.Capacity = q.String()
	}
	if pv.Spec.ClaimRef != nil {
		v.Claim = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
	}
	if pv.Spec.VolumeMode != nil {
		v.VolumeMode = string(*pv.Spec.VolumeMode)
	}
	if pv.DeletionTimestamp != nil {
		v.Status = "Terminating" // as kubectl shows it
	}
	return v
}

// accessModes writes access modes the short way kubectl does: RWO, ROX,
// RWX, RWOP, joined with commas.
func accessModes(modes []corev1.PersistentVolumeAccessMode) string {
	short := map[corev1.PersistentVolumeAccessMode]string{
		corev1.ReadWriteOnce:    "RWO",
		corev1.ReadOnlyMany:     "ROX",
		corev1.ReadWriteMany:    "RWX",
		corev1.ReadWriteOncePod: "RWOP",
	}
	out := make([]string, 0, len(modes))
	for _, m := range modes {
		if s, ok := short[m]; ok {
			out = append(out, s)
		} else {
			out = append(out, string(m))
		}
	}
	return strings.Join(out, ",")
}

// DescribePersistentVolume returns what `kubectl describe persistentvolume
// NAME` prints, using kubectl's own describer: its claim, capacity, source
// (where the storage is), node affinity and events.
func (c *Client) DescribePersistentVolume(_ context.Context, name string, showEvents bool) (string, error) {
	d := &describe.PersistentVolumeDescriber{Interface: c.clientset}
	return d.Describe("", name, describe.DescriberSettings{ShowEvents: showEvents, ChunkSize: 500})
}

// PersistentVolumeLabelsAndAnnotations returns the volume's labels and
// annotations, as key=value sorted by key.
func (c *Client) PersistentVolumeLabelsAndAnnotations(ctx context.Context, name string) (labels, annotations []string, err error) {
	return labelsAndAnnotations[corev1.PersistentVolume](ctx, c.clientset.CoreV1().PersistentVolumes(), name)
}

// LabelPersistentVolume sets and removes labels on the volume, like
// `kubectl label persistentvolume NAME env=prod old-`. A claim can pick
// a volume by its labels (its selector).
func (c *Client) LabelPersistentVolume(ctx context.Context, name string, changes []string, overwrite, dryRun bool) (Change, error) {
	return labelObject[corev1.PersistentVolume](ctx, c.clientset.CoreV1().PersistentVolumes(), name, changes, overwrite, dryRun)
}

// SetReclaimPolicy sets what happens to the volume when its claim is
// deleted, like `kubectl patch persistentvolume NAME -p
// '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}'`: Retain keeps it
// and its data (it becomes Released), Delete deletes it and, for storage
// the cluster made, the storage too.
func (c *Client) SetReclaimPolicy(ctx context.Context, name string, policy corev1.PersistentVolumeReclaimPolicy, dryRun bool) (Change, error) {
	if policy != corev1.PersistentVolumeReclaimRetain && policy != corev1.PersistentVolumeReclaimDelete {
		return Change{}, fmt.Errorf("reclaim policy %q: it is Retain or Delete", policy)
	}
	before, after, err := update(ctx, c.clientset.CoreV1().PersistentVolumes(), name, dryRun, func(pv *corev1.PersistentVolume) error {
		pv.Spec.PersistentVolumeReclaimPolicy = policy
		return nil
	})
	if err != nil {
		return Change{}, err
	}
	text := func(pv *corev1.PersistentVolume) []string {
		return []string{"reclaimPolicy=" + string(pv.Spec.PersistentVolumeReclaimPolicy)}
	}
	return Change{Before: text(before), After: text(after)}, nil
}

// MakeAvailable frees a Released volume for a new claim, like `kubectl patch
// persistentvolume NAME --type=json -p
// '[{"op":"remove","path":"/spec/claimRef"}]'`: it forgets the claim it was
// bound to, which was deleted, and becomes Available. Its data stays: the
// next claim bound to it gets it as it is.
//
// A Bound volume is refused: its claim is still there, and would lose its
// storage.
func (c *Client) MakeAvailable(ctx context.Context, name string, dryRun bool) error {
	_, _, err := update(ctx, c.clientset.CoreV1().PersistentVolumes(), name, dryRun, func(pv *corev1.PersistentVolume) error {
		switch {
		case pv.Spec.ClaimRef == nil:
			return errors.New("it has no claim to forget: it is already free")
		case pv.Status.Phase == corev1.VolumeBound:
			return fmt.Errorf("it is bound to %s/%s: delete that claim first", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		}
		pv.Spec.ClaimRef = nil
		return nil
	})
	return err
}

// DeletePersistentVolume deletes the volume, like `kubectl delete
// persistentvolume NAME`. While a claim is bound to it, it stays
// Terminating until the claim is deleted (the kubernetes.io/pv-protection
// finalizer).
func (c *Client) DeletePersistentVolume(ctx context.Context, name string, dryRun bool) error {
	return deleteObject(ctx, c.clientset.CoreV1().PersistentVolumes(), name, dryRun)
}

// CreateHostPathVolume makes a persistent volume of size, ReadWriteOnce,
// whose storage is the directory dir on the node the pod using it runs on
// (hostPath), made if it isn't there. This is storage for trying things
// out on a one-node or kind cluster: on a real cluster each node has its
// own directory. It is the YAML `kubectl apply -f` would take for:
//
//	kind: PersistentVolume
//	metadata: {name: NAME}
//	spec:
//	  capacity: {storage: SIZE}
//	  accessModes: [ReadWriteOnce]
//	  persistentVolumeReclaimPolicy: Retain
//	  storageClassName: ""          # only claims asking for no class get it
//	  hostPath: {path: DIR, type: DirectoryOrCreate}
func (c *Client) CreateHostPathVolume(ctx context.Context, name, size, dir string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("volume name %q: %s", name, strings.Join(errs, "; "))
	}
	capacity, err := resource.ParseQuantity(size)
	if err != nil || capacity.Sign() <= 0 {
		return fmt.Errorf("size %q: type an amount of storage, e.g. 1Gi or 500Mi", size)
	}
	if !path.IsAbs(dir) {
		return fmt.Errorf("path %q: type a directory from /, e.g. /mnt/data", dir)
	}
	kind := corev1.HostPathDirectoryOrCreate
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: capacity},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: path.Clean(dir), Type: &kind},
			},
		},
	}
	_, err = c.clientset.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
