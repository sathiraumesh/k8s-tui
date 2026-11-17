package kubernetes

import (
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
	// of kubectl's describers can only be made from these, and the lists
	// of the generic views are read with them (generic.go). It is nil for a
	// pretend cluster in tests.
	config *rest.Config
}

// NewClientFromClientset wraps an existing clientset, such as the fake one
// from k8s.io/client-go/kubernetes/fake, so tests can use a pretend cluster.
func NewClientFromClientset(cs kubernetes.Interface) *Client {
	return &Client{clientset: cs}
}

// NewClientFromClientsets is NewClientFromClientset with a dynamic client
// too, such as the fake one from k8s.io/client-go/dynamic/fake, for tests
// of the generic views (generic.go).
func NewClientFromClientsets(cs kubernetes.Interface, dyn dynamic.Interface) *Client {
	return &Client{clientset: cs, dynamic: dyn}
}

// NewClientFromConfig connects with the connection settings config, as
// NewClient does once it has read them. kubeContext is the kubeconfig
// context they are from, "" for none (tests against a pretend server).
func NewClientFromConfig(config *rest.Config, kubeContext string) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{clientset: clientset, dynamic: dyn, config: config, context: kubeContext}, nil
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
	return NewClientFromConfig(config, raw.CurrentContext)
}

// Context is the name of the kubeconfig context the client talks to, e.g.
// "kubernetes-admin@kubernetes", or "" if it has none (a pretend cluster).
func (c *Client) Context() string {
	return c.context
}
