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

	c9sAPIVersion         = "c9s.run/v1alpha1"
	labelRuntime          = "containerlab.dev/runtime"
	labelApp              = "c9s.run/app"
	labelTopologyOwner    = "c9s.run/topologyOwner"
	labelTopologyNode     = "c9s.run/topologyNode"
	labelIgnoreReconcile  = "c9s.run/ignoreReconcile"
	clabernetesAppValue   = "clabernetes"
	restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"
)

var topologyGVR = schema.GroupVersionResource{
	Group:    "c9s.run",
	Version:  "v1alpha1",
	Resource: "topologies",
}

var nodeGVR = schema.GroupVersionResource{
	Group:    "c9s.run",
	Version:  "v1alpha1",
	Resource: "nodes",
}

var linkGVR = schema.GroupVersionResource{
	Group:    "c9s.run",
	Version:  "v1alpha1",
	Resource: "links",
}

var launcherProfileGVR = schema.GroupVersionResource{
	Group:    "c9s.run",
	Version:  "v1alpha1",
	Resource: "launcherprofiles",
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
