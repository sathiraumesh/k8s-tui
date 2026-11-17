package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubectl/pkg/describe"
)

// PersistentVolumeClaim is one claim (PVC): a request for storage in a
// namespace, which the cluster binds to a persistent volume (PV), making
// one with the claim's storage class if needed. Pods mount the claim, not
// the volume. It has what `kubectl get persistentvolumeclaims -o wide`
// shows of it.
type PersistentVolumeClaim struct {
	Namespace string
	Name      string

	Status       string // Pending, Bound or Lost; Terminating while it is deleted
	Volume       string // the volume it is bound to, "" until then
	Capacity     string // what the bound volume has, e.g. "1Gi"; "" until bound
	AccessModes  string // e.g. "RWO", as for a volume; "" until bound
	StorageClass string // "" for none
	VolumeMode   string // Filesystem or Block

	CreatedAt time.Time
}

// ListPersistentVolumeClaims returns the claims in namespace, or in every
// namespace if it is "", sorted by namespace and name, like `kubectl get
// persistentvolumeclaims --all-namespaces`.
func (c *Client) ListPersistentVolumeClaims(ctx context.Context, namespace string) ([]PersistentVolumeClaim, error) {
	list, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	claims := make([]PersistentVolumeClaim, 0, len(list.Items))
	for _, pvc := range list.Items {
		claims = append(claims, toPersistentVolumeClaim(pvc))
	}
	sort.Slice(claims, func(i, j int) bool {
		if claims[i].Namespace != claims[j].Namespace {
			return claims[i].Namespace < claims[j].Namespace
		}
		return claims[i].Name < claims[j].Name
	})
	return claims, nil
}

// toPersistentVolumeClaim converts client-go's claim into ours, with its
// columns as kubectl writes them. Capacity and access modes are the bound
// volume's, from the claim's status, as kubectl shows them.
func toPersistentVolumeClaim(pvc corev1.PersistentVolumeClaim) PersistentVolumeClaim {
	c := PersistentVolumeClaim{
		Namespace:   pvc.Namespace,
		Name:        pvc.Name,
		Status:      string(pvc.Status.Phase),
		Volume:      pvc.Spec.VolumeName,
		AccessModes: accessModes(pvc.Status.AccessModes),
		VolumeMode:  string(corev1.PersistentVolumeFilesystem),
		CreatedAt:   pvc.CreationTimestamp.Time,
	}
	if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		c.Capacity = q.String()
	}
	if pvc.Spec.StorageClassName != nil {
		c.StorageClass = *pvc.Spec.StorageClassName
	}
	if pvc.Spec.VolumeMode != nil {
		c.VolumeMode = string(*pvc.Spec.VolumeMode)
	}
	if pvc.DeletionTimestamp != nil {
		c.Status = "Terminating" // as kubectl shows it
	}
	return c
}

// DescribePersistentVolumeClaim returns what `kubectl describe
// persistentvolumeclaim NAME -n NAMESPACE` prints, using kubectl's own
// describer: its volume, class, capacity, the pods using it ("Used By")
// and events.
func (c *Client) DescribePersistentVolumeClaim(_ context.Context, namespace, name string, showEvents bool) (string, error) {
	d := &describe.PersistentVolumeClaimDescriber{Interface: c.clientset}
	return d.Describe(namespace, name, describe.DescriberSettings{ShowEvents: showEvents, ChunkSize: 500})
}

// PersistentVolumeClaimLabelsAndAnnotations returns the claim's labels and
// annotations, as key=value sorted by key.
func (c *Client) PersistentVolumeClaimLabelsAndAnnotations(ctx context.Context, namespace, name string) (labels, annotations []string, err error) {
	return labelsAndAnnotations[corev1.PersistentVolumeClaim](ctx, c.clientset.CoreV1().PersistentVolumeClaims(namespace), name)
}

// PersistentVolumeClaimUsers lists the pods of the namespace that mount
// the claim, one line each: "NAME (PHASE) on NODE". While one does, the
// claim can't be deleted for good (the kubernetes.io/pvc-protection
// finalizer).
func (c *Client) PersistentVolumeClaimUsers(ctx context.Context, namespace, name string) ([]string, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var users []string
	for _, p := range pods.Items {
		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == name {
				user := fmt.Sprintf("%s (%s)", p.Name, p.Status.Phase)
				if p.Spec.NodeName != "" {
					user += " on " + p.Spec.NodeName
				}
				users = append(users, user)
				break
			}
		}
	}
	sort.Strings(users)
	return users, nil
}

// LabelPersistentVolumeClaim sets and removes labels on the claim, like
// `kubectl label persistentvolumeclaim NAME -n NAMESPACE env=prod old-`.
func (c *Client) LabelPersistentVolumeClaim(ctx context.Context, namespace, name string, changes []string, overwrite, dryRun bool) (Change, error) {
	return labelObject[corev1.PersistentVolumeClaim](ctx, c.clientset.CoreV1().PersistentVolumeClaims(namespace), name, changes, overwrite, dryRun)
}

// ResizePersistentVolumeClaim asks for more storage, like `kubectl patch
// persistentvolumeclaim NAME -n NAMESPACE -p
// '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'`. A claim can only
// grow, and only if its storage class allows it (allowVolumeExpansion);
// the cluster refuses it otherwise. It returns the request before and
// after.
func (c *Client) ResizePersistentVolumeClaim(ctx context.Context, namespace, name, size string, dryRun bool) (Change, error) {
	want, err := parseSize(size)
	if err != nil {
		return Change{}, err
	}
	before, after, err := update(ctx, c.clientset.CoreV1().PersistentVolumeClaims(namespace), name, dryRun, func(pvc *corev1.PersistentVolumeClaim) error {
		if have := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; want.Cmp(have) <= 0 {
			return fmt.Errorf("it asks for %s already: a claim can only grow", have.String())
		}
		if pvc.Spec.Resources.Requests == nil {
			pvc.Spec.Resources.Requests = corev1.ResourceList{}
		}
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = want
		return nil
	})
	if err != nil {
		return Change{}, err
	}
	request := func(pvc *corev1.PersistentVolumeClaim) []string {
		q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		return []string{"storage=" + q.String()}
	}
	return Change{Before: request(before), After: request(after)}, nil
}

// DeletePersistentVolumeClaim deletes the claim, like `kubectl delete
// persistentvolumeclaim NAME -n NAMESPACE`. While a pod uses it, it waits
// as Terminating. Its volume then goes by its reclaim policy: Delete
// deletes it, Retain keeps it as Released.
func (c *Client) DeletePersistentVolumeClaim(ctx context.Context, namespace, name string, dryRun bool) error {
	return deleteObject(ctx, c.clientset.CoreV1().PersistentVolumeClaims(namespace), name, dryRun)
}

// NoStorageClass is the class to give CreatePersistentVolumeClaim for a
// claim with no storage class (storageClassName: ""), which only binds to
// a volume made by hand without a class, such as a hostPath one.
const NoStorageClass = "-"

// CreatePersistentVolumeClaim makes a claim for size of storage,
// ReadWriteOnce, in namespace. class is the storage class to make its
// volume with: "" for the cluster's default class, NoStorageClass for
// none. kubectl has no command for it: it is the YAML `kubectl apply -f`
// would take for
//
//	kind: PersistentVolumeClaim
//	metadata: {name: NAME, namespace: NAMESPACE}
//	spec:
//	  accessModes: [ReadWriteOnce]
//	  resources: {requests: {storage: SIZE}}
//	  storageClassName: CLASS       # left out for the default class
func (c *Client) CreatePersistentVolumeClaim(ctx context.Context, namespace, name, size, class string, dryRun bool) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("claim name %q: %s", name, strings.Join(errs, "; "))
	}
	storage, err := parseSize(size)
	if err != nil {
		return err
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: storage},
			},
		},
	}
	switch class {
	case "":
		// left out: the cluster's default class
	case NoStorageClass:
		none := ""
		pvc.Spec.StorageClassName = &none
	default:
		pvc.Spec.StorageClassName = &class
	}
	_, err = c.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}

// parseSize reads an amount of storage as typed, e.g. "1Gi" or "500Mi".
func parseSize(size string) (resource.Quantity, error) {
	q, err := resource.ParseQuantity(strings.TrimSpace(size))
	if err != nil || q.Sign() <= 0 {
		return resource.Quantity{}, fmt.Errorf("size %q: type an amount of storage, e.g. 1Gi or 500Mi", size)
	}
	return q, nil
}
