package clabernetes

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const labNamespacePrefix = "c9s-"

func configuredLabNamespace(namespace string) string {
	if namespace != "" {
		return namespace
	}

	return os.Getenv(envNamespace)
}

func (r *Runtime) namespaceFor(namespace string) string {
	if namespace != "" {
		return namespace
	}
	if r.namespace != "" {
		return r.namespace
	}
	return defaultNamespace
}

func canonicalNamespaceForLab(name string) (string, error) {
	namespace := labNamespacePrefix + name
	if errs := validation.IsDNS1123Label(namespace); len(errs) != 0 {
		return "", fmt.Errorf(
			"cannot derive Kubernetes namespace for c9s lab %q: %s",
			name,
			errs[0],
		)
	}

	return namespace, nil
}

func (r *Runtime) namespaceForLab(name, namespace string) (string, error) {
	if namespace != "" {
		return namespace, nil
	}
	if r.labNamespaceOverride != "" {
		return r.labNamespaceOverride, nil
	}

	return canonicalNamespaceForLab(name)
}

func (r *Runtime) ensureLabNamespace(
	ctx context.Context,
	name,
	namespace string,
	managed bool,
) (bool, error) {
	namespaces := r.kubeClient.CoreV1().Namespaces()
	existing, err := namespaces.Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		if owner := existing.Labels[labelTopologyOwner]; managed && owner != "" && owner != name {
			return false, fmt.Errorf(
				"c9s namespace %q belongs to lab %q, not %q",
				namespace,
				owner,
				name,
			)
		}

		return false, nil
	}
	if !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to get c9s namespace %q: %w", namespace, err)
	}
	if !managed {
		return false, fmt.Errorf(
			"c9s namespace override %q does not exist; create it before deploying the lab",
			namespace,
		)
	}

	_, err = namespaces.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				labelRuntime:       clabernetesAppValue,
				labelTopologyOwner: name,
				// The c9s connectivity sidecar is privileged; without this label a cluster
				// enforcing a restrictive Pod Security default rejects every device pod.
				// clabverter stamps the same label on the namespaces it emits.
				"pod-security.kubernetes.io/enforce": "privileged",
			},
		},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, err = namespaces.Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf(
				"failed to get concurrently created c9s namespace %q: %w",
				namespace,
				err,
			)
		}
		if owner := existing.Labels[labelTopologyOwner]; owner != "" && owner != name {
			return false, fmt.Errorf(
				"c9s namespace %q belongs to lab %q, not %q",
				namespace,
				owner,
				name,
			)
		}

		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to create c9s namespace %q: %w", namespace, err)
	}

	return true, nil
}

func (r *Runtime) deleteManagedLabNamespace(
	ctx context.Context,
	name,
	namespace string,
) (bool, error) {
	canonicalNamespace, err := canonicalNamespaceForLab(name)
	if err != nil || namespace != canonicalNamespace {
		return false, nil
	}

	namespaces := r.kubeClient.CoreV1().Namespaces()
	existing, err := namespaces.Get(ctx, namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get c9s namespace %q: %w", namespace, err)
	}
	if existing.Labels[labelRuntime] != clabernetesAppValue ||
		existing.Labels[labelTopologyOwner] != name {
		return false, nil
	}

	err = namespaces.Delete(ctx, namespace, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to delete c9s namespace %q: %w", namespace, err)
	}

	return true, nil
}

func (r *Runtime) waitNamespaceDeleted(
	ctx context.Context,
	namespace string,
	timeout time.Duration,
) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	for {
		_, err := r.kubeClient.CoreV1().Namespaces().Get(
			waitCtx,
			namespace,
			metav1.GetOptions{},
		)
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get c9s namespace %q: %w", namespace, err)
		}

		select {
		case <-waitCtx.Done():
			return fmt.Errorf(
				"timed out after %s waiting for c9s namespace %q to be deleted",
				r.timeoutFor(timeout),
				namespace,
			)
		case <-time.After(pollInterval):
		}
	}
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
