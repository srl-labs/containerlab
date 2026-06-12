package clabernetes

import (
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func (r *Runtime) namespaceFor(namespace string) string {
	if namespace != "" {
		return namespace
	}
	if r.namespace != "" {
		return r.namespace
	}
	return defaultNamespace
}

func (r *Runtime) timeoutFor(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if r.timeout > 0 {
		return r.timeout
	}
	return 10 * time.Minute
}

func kubeClientConfig() (*rest.Config, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig := os.Getenv(envKubeconfig); kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	overrides := &clientcmd.ConfigOverrides{}
	if contextName := os.Getenv(envContext); contextName != "" {
		overrides.CurrentContext = contextName
	}
	if namespace := os.Getenv(envNamespace); namespace != "" {
		overrides.Context.Namespace = namespace
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		overrides,
	)

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		namespace = defaultNamespace
	}

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load Kubernetes client config: %w", err)
	}

	return restConfig, namespace, nil
}
