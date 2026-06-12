package clabernetes

import (
	"fmt"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultNamespace = "default"
	pollInterval     = 2 * time.Second

	envKubeconfig = "CLAB_KUBECONFIG"
	envContext    = "CLAB_KUBE_CONTEXT"
	envNamespace  = "CLAB_KUBE_NAMESPACE"

	labelApp              = "clabernetes/app"
	labelTopologyOwner    = "clabernetes/topologyOwner"
	labelTopologyNode     = "clabernetes/topologyNode"
	labelIgnoreReconcile  = "clabernetes/ignoreReconcile"
	clabernetesAppValue   = "clabernetes"
	restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"
)

var topologyGVR = schema.GroupVersionResource{
	Group:    "clabernetes.containerlab.dev",
	Version:  "v1alpha1",
	Resource: "topologies",
}

type Runtime struct {
	client     dynamic.Interface
	kubeClient kubernetes.Interface
	restConfig *rest.Config
	namespace  string
	timeout    time.Duration
}

func init() {
	clablabruntime.Register(clablabruntime.ClabernetesRuntimeName, New)
}

func New(cfg clablabruntime.Config) (clablabruntime.LabRuntime, error) {
	kubeConfig, namespace, err := kubeClientConfig()
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes dynamic client: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	if namespace == "" {
		namespace = defaultNamespace
	}

	return &Runtime{
		client:     client,
		kubeClient: kubeClient,
		restConfig: kubeConfig,
		namespace:  namespace,
		timeout:    cfg.Timeout,
	}, nil
}
