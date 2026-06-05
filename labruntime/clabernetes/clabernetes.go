package clabernetes

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"time"

	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/labruntime"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultNamespace = "default"
	pollInterval     = 2 * time.Second

	envKubeconfig = "CLAB_KUBECONFIG"
	envContext    = "CLAB_KUBE_CONTEXT"
	envNamespace  = "CLAB_KUBE_NAMESPACE"
)

var topologyGVR = schema.GroupVersionResource{
	Group:    "clabernetes.containerlab.dev",
	Version:  "v1alpha1",
	Resource: "topologies",
}

type Runtime struct {
	client    dynamic.Interface
	namespace string
	timeout   time.Duration
}

func init() {
	labruntime.Register(labruntime.ClabernetesRuntimeName, New)
}

func New(cfg labruntime.Config) (labruntime.LabRuntime, error) {
	kubeConfig, namespace, err := kubeClientConfig()
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes dynamic client: %w", err)
	}

	if namespace == "" {
		namespace = defaultNamespace
	}

	return &Runtime{
		client:    client,
		namespace: namespace,
		timeout:   cfg.Timeout,
	}, nil
}

func (r *Runtime) Capabilities() labruntime.RuntimeCapabilities {
	return labruntime.RuntimeCapabilities{
		Deploy:  true,
		Destroy: true,
		Inspect: true,
		List:    true,
	}
}

func (r *Runtime) Deploy(
	ctx context.Context,
	req labruntime.DeployRequest,
) (*labruntime.LabState, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("topology name is required")
	}

	if len(req.TopologyDefinition) == 0 {
		return nil, fmt.Errorf("rendered containerlab topology is required")
	}

	namespace := r.namespaceFor(req.Namespace)
	resource := r.client.Resource(topologyGVR).Namespace(namespace)
	desired := topologyObject(req.Name, namespace, string(req.TopologyDefinition))

	existing, err := resource.Get(ctx, req.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		log.Info("Creating clabernetes topology", "name", req.Name, "namespace", namespace)
		_, err = resource.Create(ctx, desired, metav1.CreateOptions{})
	case err != nil:
		return nil, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	default:
		log.Info("Updating clabernetes topology", "name", req.Name, "namespace", namespace)
		desired.SetResourceVersion(existing.GetResourceVersion())
		_, err = resource.Update(ctx, desired, metav1.UpdateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to apply clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	if !req.Wait {
		return r.Inspect(ctx, labruntime.InspectRequest{Name: req.Name, Namespace: namespace})
	}

	if err := r.waitReady(ctx, req.Name, namespace, req.Timeout); err != nil {
		return nil, err
	}

	return r.Inspect(ctx, labruntime.InspectRequest{Name: req.Name, Namespace: namespace})
}

func (r *Runtime) Destroy(ctx context.Context, req labruntime.DestroyRequest) error {
	if req.Name == "" {
		return fmt.Errorf("topology name is required")
	}

	namespace := r.namespaceFor(req.Namespace)
	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	log.Info("Deleting clabernetes topology", "name", req.Name, "namespace", namespace)

	err := resource.Delete(ctx, req.Name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		log.Info("clabernetes topology not found", "name", req.Name, "namespace", namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	if !req.Wait {
		return nil
	}

	return r.waitDeleted(ctx, req.Name, namespace, req.Timeout)
}

func (r *Runtime) Inspect(
	ctx context.Context,
	req labruntime.InspectRequest,
) (*labruntime.LabState, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("topology name is required")
	}

	namespace := r.namespaceFor(req.Namespace)
	obj, err := r.client.Resource(topologyGVR).Namespace(namespace).
		Get(ctx, req.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	return stateFromTopology(obj, namespace), nil
}

func (r *Runtime) List(
	ctx context.Context,
	req labruntime.ListRequest,
) ([]*labruntime.LabState, error) {
	namespace := r.namespaceFor(req.Namespace)
	if req.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	list, err := r.client.Resource(topologyGVR).Namespace(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes topologies: %w", err)
	}

	states := make([]*labruntime.LabState, 0, len(list.Items))
	for idx := range list.Items {
		states = append(states, stateFromTopology(&list.Items[idx], namespace))
	}

	sort.Slice(states, func(i, j int) bool {
		if states[i].Namespace == states[j].Namespace {
			return states[i].Name < states[j].Name
		}
		return states[i].Namespace < states[j].Namespace
	})

	return states, nil
}

func (r *Runtime) waitReady(ctx context.Context, name, namespace string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	return wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			obj, err := resource.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
					namespace, name, err)
			}

			state := stateFromTopology(obj, namespace)
			if state.Ready {
				return true, nil
			}
			if state.State == "deployfailed" {
				return false, fmt.Errorf("clabernetes topology %s/%s reported deployfailed",
					namespace, name)
			}

			log.Debug("Waiting for clabernetes topology",
				"name", name,
				"namespace", namespace,
				"state", state.State,
			)

			return false, nil
		})
}

func (r *Runtime) waitDeleted(ctx context.Context, name, namespace string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	return wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			_, err := resource.Get(ctx, name, metav1.GetOptions{})
			switch {
			case apierrors.IsNotFound(err):
				return true, nil
			case err != nil:
				return false, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
					namespace, name, err)
			default:
				return false, nil
			}
		})
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

func (r *Runtime) timeoutFor(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if r.timeout > 0 {
		return r.timeout
	}
	return 10 * time.Minute
}

func topologyObject(name, namespace, definition string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "clabernetes.containerlab.dev/v1alpha1",
			"kind":       "Topology",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]any{
					"containerlab.dev/runtime": labruntime.ClabernetesRuntimeName,
				},
			},
			"spec": map[string]any{
				"definition": map[string]any{
					"containerlab": definition,
				},
			},
		},
	}
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

func stateFromTopology(obj *unstructured.Unstructured, namespace string) *labruntime.LabState {
	if obj.GetNamespace() != "" {
		namespace = obj.GetNamespace()
	}

	ready, _, _ := unstructured.NestedBool(obj.Object, "status", "topologyReady")
	state, _, _ := unstructured.NestedString(obj.Object, "status", "topologyState")
	nodeReadiness, _, _ := unstructured.NestedStringMap(
		obj.Object,
		"status",
		"nodeReadiness",
	)
	exposedPorts, _, _ := unstructured.NestedMap(obj.Object, "status", "exposedPorts")
	nodeSpecs := nodeSpecsFromTopology(obj)

	nodeNames := make([]string, 0, len(nodeSpecs)+len(nodeReadiness))
	seenNodes := map[string]struct{}{}
	for nodeName := range nodeSpecs {
		nodeNames = append(nodeNames, nodeName)
		seenNodes[nodeName] = struct{}{}
	}
	for nodeName := range nodeReadiness {
		if _, ok := seenNodes[nodeName]; ok {
			continue
		}
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	nodes := make([]labruntime.NodeState, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		nodeState := nodeReadiness[nodeName]
		spec := nodeSpecs[nodeName]
		nodes = append(nodes, labruntime.NodeState{
			Name:                nodeName,
			Kind:                spec.Kind,
			Image:               spec.Image,
			State:               nodeState,
			Ready:               nodeState == "ready",
			LoadBalancerAddress: loadBalancerAddress(exposedPorts, nodeName),
		})
	}

	return &labruntime.LabState{
		Name:         obj.GetName(),
		Namespace:    namespace,
		TopologyPath: fmt.Sprintf("k8s://%s/topologies/%s", namespace, obj.GetName()),
		State:        state,
		Ready:        ready,
		Nodes:        nodes,
	}
}

type nodeSpec struct {
	Kind  string `yaml:"kind"`
	Image string `yaml:"image"`
}

type containerlabDefinition struct {
	Topology struct {
		Nodes map[string]nodeSpec `yaml:"nodes"`
	} `yaml:"topology"`
}

func nodeSpecsFromTopology(obj *unstructured.Unstructured) map[string]nodeSpec {
	specs := map[string]nodeSpec{}

	statusConfigs, _, _ := unstructured.NestedStringMap(obj.Object, "status", "configs")
	for _, config := range statusConfigs {
		mergeNodeSpecs(specs, config)
	}

	if len(specs) != 0 {
		return specs
	}

	definition, _, _ := unstructured.NestedString(
		obj.Object,
		"spec",
		"definition",
		"containerlab",
	)
	mergeNodeSpecs(specs, definition)

	return specs
}

func mergeNodeSpecs(specs map[string]nodeSpec, definition string) {
	if definition == "" {
		return
	}

	var parsed containerlabDefinition
	if err := yaml.Unmarshal([]byte(definition), &parsed); err != nil {
		log.Debug("failed to parse clabernetes topology definition", "error", err)
		return
	}

	for nodeName, spec := range parsed.Topology.Nodes {
		specs[nodeName] = spec
	}
}

func loadBalancerAddress(exposedPorts map[string]any, nodeName string) string {
	raw, ok := exposedPorts[nodeName]
	if !ok {
		return ""
	}

	nodeExpose, ok := raw.(map[string]any)
	if !ok {
		return ""
	}

	addr, ok := nodeExpose["loadBalancerAddress"].(string)
	if !ok {
		return ""
	}

	if net.ParseIP(addr) == nil {
		return ""
	}

	return addr
}
