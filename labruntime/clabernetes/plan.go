package clabernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

type preparedDeployment struct {
	topology   *unstructured.Unstructured
	configMaps []stagedConfigMap
	primitives *primitiveResourceSet
}

func prepareDesiredDeployment(
	req clablabruntime.DeployRequest,
	namespace string,
) (*preparedDeployment, error) {
	topologyDefinition, stagedConfigMaps, naming, err := stageTopologyLocalFiles(req)
	if err != nil {
		return nil, err
	}

	desiredTopology := topologyObject(
		req.Name,
		namespace,
		req.Owner,
		string(topologyDefinition),
		topologyWithNaming(naming),
		topologyWithImagePullSecret(req.ImagePullSecret),
	)
	if err := setTopologyFilesFromConfigMaps(desiredTopology, stagedConfigMaps); err != nil {
		return nil, err
	}
	primitives, err := compilePrimitiveResources(desiredTopology)
	if err != nil {
		return nil, err
	}

	if err := validatePrimitiveNodeKinds(primitives); err != nil {
		return nil, err
	}
	if err := validatePrimitiveNodeNames(primitives); err != nil {
		return nil, err
	}

	return &preparedDeployment{
		topology:   desiredTopology,
		configMaps: stagedConfigMaps,
		primitives: primitives,
	}, nil
}

// unsupportedC9sNodeKinds names containerlab pseudo-node kinds that reference host-machine
// constructs a Kubernetes pod cannot provide. The c9s compiler treats kinds as opaque, so the
// runtime rejects them client-side before anything is created.
var unsupportedC9sNodeKinds = map[string]string{ //nolint:gochecknoglobals
	"bridge":        "host Linux bridges do not exist in a Kubernetes pod network",
	"ovs-bridge":    "host Open vSwitch bridges do not exist in a Kubernetes pod network",
	"host":          "the containerlab host machine is not part of a Kubernetes cluster",
	"ext-container": "externally managed containers are not reachable from c9s pods",
	"k8s-kind":      "nested kind clusters are not supported by the c9s runtime",
}

// validatePrimitiveNodeKinds rejects pseudo-node kinds that cannot be realized by c9s.
func validatePrimitiveNodeKinds(primitives *primitiveResourceSet) error {
	var unsupported []string
	for _, node := range primitives.nodes {
		kind, _, _ := unstructured.NestedString(node.Object, "spec", "kind")
		reason, isUnsupported := unsupportedC9sNodeKinds[kind]
		if !isUnsupported {
			continue
		}
		unsupported = append(unsupported, fmt.Sprintf(
			"node %q uses pseudo-node kind %q: %s", node.GetName(), kind, reason))
	}
	if len(unsupported) == 0 {
		return nil
	}
	sort.Strings(unsupported)

	return fmt.Errorf("topology contains features unsupported by c9s: %s",
		strings.Join(unsupported, "; "))
}

// validatePrimitiveNodeNames rejects node names Kubernetes cannot carry. c9s uses containerlab
// node names verbatim as Node object, Deployment, and Service names, so every node name must be
// a DNS-1035 label. Node names are sanitized while the topology is staged, so this is the
// backstop that keeps the deploy fail-closed instead of surfacing an opaque API-server rejection
// after some resources were already created.
func validatePrimitiveNodeNames(primitives *primitiveResourceSet) error {
	var invalid []string
	for _, node := range primitives.nodes {
		name := node.GetName()
		if errs := validation.IsDNS1035Label(name); len(errs) != 0 {
			invalid = append(invalid, fmt.Sprintf("%s (%s)", name, errs[0]))
		}
	}
	if len(invalid) == 0 {
		return nil
	}
	sort.Strings(invalid)

	return fmt.Errorf(
		"c9s node names must be lowercase RFC 1035 labels "+
			"(start with a letter, contain only a-z, 0-9 and '-'): %s",
		strings.Join(invalid, "; "),
	)
}

// Validate compiles the containerlab/c9s subset and stages all local-path inputs in memory.
// Lossy-but-deployable fields produce warnings; structurally impossible constructs remain
// errors. Validation deliberately performs no Kubernetes reads or writes.
func (r *Runtime) Validate(_ context.Context, req clablabruntime.DeployRequest) error {
	if req.Name == "" {
		return fmt.Errorf("topology name is required")
	}
	if len(req.TopologyDefinition) == 0 {
		return fmt.Errorf("rendered containerlab topology is required")
	}

	namespace, err := r.namespaceForLab(req.Name, req.Namespace)
	if err != nil {
		return err
	}

	_, err = prepareDesiredDeployment(req, namespace)

	return err
}

// Plan compiles the c9s topology and returns the complete primitive/configuration diff against
// the running cluster without changing any resource.
func (r *Runtime) Plan(
	ctx context.Context,
	req clablabruntime.DeployRequest,
) (*clablabruntime.DeployPlan, error) {
	if err := r.Validate(ctx, req); err != nil {
		return nil, err
	}

	namespace, err := r.namespaceForLab(req.Name, req.Namespace)
	if err != nil {
		return nil, err
	}
	prepared, err := prepareDesiredDeployment(req, namespace)
	if err != nil {
		return nil, err
	}

	plan := &clablabruntime.DeployPlan{
		LabName: req.Name, Namespace: namespace, Changes: []clablabruntime.ResourceChange{},
	}
	namespaceExists, err := r.planNamespace(ctx, req, namespace, plan)
	if err != nil {
		return nil, err
	}
	if !namespaceExists {
		appendAllDesiredCreates(plan, namespace, prepared, req.NoTopologyCR)
		sortDeployPlan(plan)

		return plan, nil
	}

	existingTopology, err := r.client.Resource(topologyGVR).Namespace(namespace).
		Get(ctx, req.Name, metav1.GetOptions{})
	if err == nil {
		if req.NoTopologyCR {
			return nil, fmt.Errorf(
				"clabernetes lab %s/%s is owned by a Topology resource; "+
					"deploy without --no-topology-cr to reconcile it, or destroy the lab first",
				namespace, req.Name)
		}

		updated := reconciledPrimitiveObject(existingTopology, prepared.topology)
		if !primitiveObjectsConform(existingTopology, updated) {
			appendPlanChange(plan, clablabruntime.ChangeUpdate, "Topology", namespace, req.Name)
		}
		if err := r.planConfigMaps(
			ctx,
			namespace,
			req.Name,
			prepared.configMaps,
			plan,
		); err != nil {
			return nil, err
		}
		sortDeployPlan(plan)

		return plan, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			namespace, req.Name, err)
	}

	if req.NoTopologyCR {
		err := r.planPrimitiveResources(ctx, namespace, req.Name, prepared.primitives, plan)
		if err != nil {
			return nil, err
		}
	} else {
		// The controller compiles and owns the primitive resources once the Topology exists, so
		// the plan reports only the resources containerlab itself would create.
		appendPlanChange(plan, clablabruntime.ChangeCreate, "Topology", namespace, req.Name)
	}
	if err := r.planConfigMaps(ctx, namespace, req.Name, prepared.configMaps, plan); err != nil {
		return nil, err
	}
	sortDeployPlan(plan)

	return plan, nil
}

func (r *Runtime) planNamespace(
	ctx context.Context,
	req clablabruntime.DeployRequest,
	namespace string,
	plan *clablabruntime.DeployPlan,
) (bool, error) {
	existing, err := r.kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	managed := req.Namespace == "" && r.labNamespaceOverride == ""
	if err == nil {
		if owner := existing.Labels[labelTopologyOwner]; managed && owner != "" &&
			owner != req.Name {
			return false, fmt.Errorf("c9s namespace %q belongs to lab %q, not %q",
				namespace, owner, req.Name)
		}

		return true, nil
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

	appendPlanChange(plan, clablabruntime.ChangeCreate, "Namespace", "", namespace)

	return false, nil
}

func appendAllDesiredCreates(
	plan *clablabruntime.DeployPlan,
	namespace string,
	prepared *preparedDeployment,
	noTopologyCR bool,
) {
	if noTopologyCR {
		for _, group := range prepared.primitives.groups() {
			for _, obj := range group.objects {
				appendPlanChange(
					plan,
					clablabruntime.ChangeCreate,
					group.kind,
					namespace,
					obj.GetName(),
				)
			}
		}
	} else {
		appendPlanChange(
			plan,
			clablabruntime.ChangeCreate,
			"Topology",
			namespace,
			prepared.topology.GetName(),
		)
	}
	for _, configMap := range prepared.configMaps {
		appendPlanChange(plan, clablabruntime.ChangeCreate, "ConfigMap", namespace, configMap.name)
	}
}

func (r *Runtime) planPrimitiveResources(
	ctx context.Context,
	namespace,
	topologyName string,
	desired *primitiveResourceSet,
	plan *clablabruntime.DeployPlan,
) error {
	existing, err := r.primitiveResourceInventory(ctx, namespace)
	if err != nil {
		return err
	}
	if err := validatePrimitiveResourceOwnership(
		existing,
		desired,
		topologyName,
		namespace,
	); err != nil {
		return err
	}

	desiredNames := map[schema.GroupVersionResource]map[string]struct{}{}
	for _, group := range desired.groups() {
		desiredNames[group.gvr] = map[string]struct{}{}
		for _, obj := range group.objects {
			desiredNames[group.gvr][obj.GetName()] = struct{}{}
			actual := existing[group.gvr][obj.GetName()]
			if actual == nil {
				appendPlanChange(
					plan,
					clablabruntime.ChangeCreate,
					group.kind,
					namespace,
					obj.GetName(),
				)
				continue
			}

			updated := reconciledPrimitiveObject(actual, obj)
			if !primitiveObjectsConform(actual, updated) {
				appendPlanChange(
					plan,
					clablabruntime.ChangeUpdate,
					group.kind,
					namespace,
					obj.GetName(),
				)
			}
		}
	}

	for _, group := range []primitiveResourceGroup{
		{gvr: linkGVR, kind: "Link"},
		{gvr: nodeGVR, kind: "Node"},
		{gvr: nodeProfileGVR, kind: "NodeProfile"},
	} {
		for name, actual := range existing[group.gvr] {
			if actual.GetLabels()[labelTopologyOwner] != topologyName {
				continue
			}
			if _, keep := desiredNames[group.gvr][name]; !keep {
				appendPlanChange(plan, clablabruntime.ChangeDelete, group.kind, namespace, name)
			}
		}
	}

	return nil
}

func (r *Runtime) planConfigMaps(
	ctx context.Context,
	namespace,
	topologyName string,
	desired []stagedConfigMap,
	plan *clablabruntime.DeployPlan,
) error {
	existingList, err := r.kubeClient.CoreV1().ConfigMaps(namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: labels.Set{labelTopologyOwner: topologyName}.String()},
	)
	if err != nil {
		return fmt.Errorf("failed to list staged ConfigMaps for c9s lab %s/%s: %w",
			namespace, topologyName, err)
	}
	existing := make(map[string]*corev1.ConfigMap, len(existingList.Items))
	for idx := range existingList.Items {
		existing[existingList.Items[idx].Name] = &existingList.Items[idx]
	}

	desiredNames := make(map[string]struct{}, len(desired))
	for _, staged := range desired {
		desiredNames[staged.name] = struct{}{}
		wanted := stagedConfigMapObject(namespace, topologyName, staged, nil)
		actual := existing[staged.name]
		if actual == nil {
			appendPlanChange(plan, clablabruntime.ChangeCreate, "ConfigMap", namespace, staged.name)
			continue
		}

		updated := actual.DeepCopy()
		updated.Labels = mergeDesiredMetadata(actual.Labels, wanted.Labels)
		updated.Data = wanted.Data
		updated.BinaryData = wanted.BinaryData
		if !stagedConfigMapsConform(actual, updated) {
			appendPlanChange(plan, clablabruntime.ChangeUpdate, "ConfigMap", namespace, staged.name)
		}
	}

	for name := range existing {
		if _, keep := desiredNames[name]; !keep {
			appendPlanChange(plan, clablabruntime.ChangeDelete, "ConfigMap", namespace, name)
		}
	}

	return nil
}

func appendPlanChange(
	plan *clablabruntime.DeployPlan,
	action clablabruntime.ChangeAction,
	kind,
	namespace,
	name string,
) {
	plan.Changes = append(plan.Changes, clablabruntime.ResourceChange{
		Action: action, Kind: kind, Namespace: namespace, Name: name,
	})
}

func sortDeployPlan(plan *clablabruntime.DeployPlan) {
	sort.Slice(plan.Changes, func(i, j int) bool {
		left, right := plan.Changes[i], plan.Changes[j]
		if left.Action != right.Action {
			return left.Action < right.Action
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Namespace != right.Namespace {
			return left.Namespace < right.Namespace
		}

		return left.Name < right.Name
	})
}
