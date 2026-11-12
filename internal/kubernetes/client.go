package kubernetes

import (
	"context"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client talks to one Kubernetes cluster. Create it once with NewClient and
// reuse it: it is safe to use from several goroutines at the same time.
type Client struct {
	clientset kubernetes.Interface

	// context is the name of the kubeconfig context the client was made
	// for, or "" if there is none.
	context string

	// dynamic reaches any kind of thing the cluster serves, also the kinds
	// the clientset has no typed access to. It is nil for a pretend
	// cluster that was not given one.
	dynamic dynamic.Interface

	// config holds the connection settings the client was made from. Some
	// of kubectl's describers can only be made from these. It is nil for a
	// pretend cluster in tests.
	config *rest.Config

	// rawGet fetches a path of the cluster's API as it is, for the parts of
	// the API the clientset has no typed access to (the metrics). It is nil
	// when there is no such access, as with a pretend cluster in tests.
	rawGet func(ctx context.Context, path string) ([]byte, error)
}
