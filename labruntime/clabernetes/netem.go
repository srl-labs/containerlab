package clabernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// linkEndpointFields are the spec fields of a c9s Link's two endpoints.
var linkEndpointFields = []string{"endpointA", "endpointB"} //nolint:gochecknoglobals

// SetImpairment sets the netem impairment of one lab link interface on the endpoint of its c9s
// Link, or removes it when the request carries none. The connectivity sidecar of the node's pod
// applies the change live; a later deploy restores the impairments the topology declares. A lab
// owned by a Topology resource is rejected: its controller renders the Links and reverts any
// direct change, so its impairments come from the topology.
func (r *Runtime) SetImpairment(
	ctx context.Context,
	req clablabruntime.ImpairmentRequest,
) error {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return err
	}

	if err = r.ensureLinkNetemSupport(ctx); err != nil {
		return err
	}

	locations, err := r.LocateNode(ctx, clablabruntime.NodeRequest{
		Name: req.Name, Namespace: req.Namespace, Nodes: []string{req.Node},
		Identity: req.Identity,
	})
	if err != nil {
		return err
	}
	location := locations[0]

	_, err = r.client.Resource(topologyGVR).Namespace(location.Namespace).
		Get(ctx, req.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		return fmt.Errorf("c9s lab %s/%s is owned by a Topology resource, whose controller "+
			"reverts changes to its links; declare the impairment in the topology and deploy "+
			"again, or deploy the lab with --no-topology-cr", location.Namespace, req.Name)
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			location.Namespace, req.Name, err)
	}

	links, err := r.primitiveResourcesForTopology(ctx, linkGVR, req.Name, location.Namespace)
	if err != nil {
		return err
	}

	link, endpoint, ok := linkEndpoint(links.Items, location.NodeName, req.Interface)
	if !ok {
		return fmt.Errorf("interface %q of node %q is not on a link of c9s lab %s/%s",
			req.Interface, req.Node, location.Namespace, req.Name)
	}

	// a merge patch with a null netem removes it
	var netem any
	if req.Impairment != nil {
		netem = linkNetem(req.Impairment)
	}

	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{endpoint: map[string]any{"netem": netem}},
	})
	if err != nil {
		return err
	}

	_, err = r.client.Resource(linkGVR).Namespace(location.Namespace).Patch(
		ctx, link.GetName(), types.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to set the impairment of c9s link %s/%s: %w",
			location.Namespace, link.GetName(), err)
	}

	return nil
}

// Impairments lists the impaired link interfaces of the requested nodes, or of every node, as
// their c9s Links declare them.
func (r *Runtime) Impairments(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) ([]clablabruntime.NodeImpairments, error) {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return nil, err
	}

	locations, err := r.LocateNode(ctx, req)
	if err != nil || len(locations) == 0 {
		return nil, err
	}

	names, err := r.nodeDisplayNames(ctx, req.Name, locations)
	if err != nil {
		return nil, err
	}

	links, err := r.client.Resource(linkGVR).Namespace(locations[0].Namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: labels.Set{labelTopologyOwner: req.Name}.String()},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list c9s links for lab %s/%s: %w",
			locations[0].Namespace, req.Name, err)
	}

	impairments := map[string][]clabtypes.ImpairmentData{}
	for _, location := range locations {
		impairments[location.NodeName] = nil
	}

	for idx := range links.Items {
		for _, endpoint := range linkEndpointFields {
			node, _, _ := unstructured.NestedString(links.Items[idx].Object,
				"spec", endpoint, "nodeName")
			iface, _, _ := unstructured.NestedString(links.Items[idx].Object,
				"spec", endpoint, "interfaceName")
			netem, found, _ := unstructured.NestedMap(links.Items[idx].Object,
				"spec", endpoint, "netem")

			if _, requested := impairments[node]; !requested || !found || len(netem) == 0 {
				continue
			}

			impairments[node] = append(impairments[node], impairmentData(iface, netem))
		}
	}

	result := make([]clablabruntime.NodeImpairments, 0, len(locations))
	for _, location := range locations {
		nodeImpairments := impairments[location.NodeName]
		sort.Slice(nodeImpairments, func(i, j int) bool {
			return nodeImpairments[i].Interface < nodeImpairments[j].Interface
		})

		result = append(result, clablabruntime.NodeImpairments{
			Node:        names.display(location.NodeName),
			Impairments: nodeImpairments,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Node < result[j].Node })

	return result, nil
}

// linksDeclareNetem reports whether any of the Links impairs one of its endpoints.
func linksDeclareNetem(links []*unstructured.Unstructured) bool {
	for _, link := range links {
		for _, endpoint := range linkEndpointFields {
			if _, found, _ := unstructured.NestedFieldNoCopy(link.Object,
				"spec", endpoint, "netem"); found {
				return true
			}
		}
	}

	return false
}

// linkEndpoint finds the c9s Link with an endpoint on the node's interface and returns it with
// the spec field of that endpoint.
func linkEndpoint(
	links []unstructured.Unstructured,
	node,
	iface string,
) (*unstructured.Unstructured, string, bool) {
	for idx := range links {
		for _, endpoint := range linkEndpointFields {
			endpointNode, _, _ := unstructured.NestedString(links[idx].Object,
				"spec", endpoint, "nodeName")
			endpointIface, _, _ := unstructured.NestedString(links[idx].Object,
				"spec", endpoint, "interfaceName")

			if endpointNode == node && endpointIface == iface {
				return &links[idx], endpoint, true
			}
		}
	}

	return nil, "", false
}

// linkNetem renders an impairment in the c9s Link vocabulary, which is containerlab's.
func linkNetem(impairment *clablabruntime.Impairment) map[string]any {
	netem := map[string]any{}

	if impairment.Delay != 0 {
		netem["delay"] = impairment.Delay.String()
	}
	if impairment.Jitter != 0 {
		netem["jitter"] = impairment.Jitter.String()
	}
	if impairment.Loss != 0 {
		netem["loss"] = impairment.Loss
	}
	if impairment.Rate != 0 {
		netem["rate"] = impairment.Rate
	}
	if impairment.Corruption != 0 {
		netem["corruption"] = impairment.Corruption
	}

	return netem
}

// impairmentData converts a c9s Link endpoint netem into the shape tools netem show reports for
// local labs.
func impairmentData(iface string, netem map[string]any) clabtypes.ImpairmentData {
	data := clabtypes.ImpairmentData{Interface: iface}

	if delay, ok := netem["delay"].(string); ok {
		data.Delay = normalizedDuration(delay)
	}
	if jitter, ok := netem["jitter"].(string); ok {
		data.Jitter = normalizedDuration(jitter)
	}

	data.PacketLoss = roundedPercent(netem["loss"])
	data.Corruption = roundedPercent(netem["corruption"])

	switch rate := netem["rate"].(type) {
	case int64:
		data.Rate = int(rate)
	case float64:
		data.Rate = int(rate)
	}

	return data
}

func normalizedDuration(value string) string {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return value
	}

	return duration.String()
}

// roundedPercent rounds a percentage to 2 decimal places, like tools netem show.
func roundedPercent(value any) float64 {
	var percent float64

	switch typed := value.(type) {
	case int64:
		percent = float64(typed)
	case float64:
		percent = typed
	}

	return math.Round(percent*100) / 100 //nolint:mnd
}
