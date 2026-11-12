package kubernetes

import (
	"context"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

// NewClient connects using your kubeconfig (~/.kube/config, or $KUBECONFIG),
// the same way kubectl does, with its current context.
func NewClient() (*Client, error) {
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	raw, err := loader.RawConfig() // to read the current context's name
	if err != nil {
		return nil, err
	}
	config, err := loader.ClientConfig() // connection settings: server, certs, token
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		clientset: clientset,
		dynamic:   dyn,
		config:    config,
		context:   raw.CurrentContext,
	}, nil
}
