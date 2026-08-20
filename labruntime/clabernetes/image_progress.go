package clabernetes

import (
	"context"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

const launcherLogTailLines int64 = 200

type imagePullRequest struct {
	name           string
	node           string
	image          string
	kubernetesNode string
	complete       bool
}

type imagePullProgress struct {
	active                    map[string]imagePullRequest
	listErrorReported         bool
	launcherListErrorReported bool
	launcherLogErrors         map[string]struct{}
	launcherCopies            map[string]launcherImageCopyProgress
}

type launcherImageLog struct {
	podName        string
	node           string
	image          string
	kubernetesNode string
	content        string
}

type launcherImageCopyProgress struct {
	copying  bool
	complete bool
}

func (r *Runtime) imagePullRequests(
	ctx context.Context,
	namespace string,
	state *clablabruntime.LabState,
) ([]imagePullRequest, error) {
	labNodes := make(map[string]string, len(state.Nodes))
	for _, node := range state.Nodes {
		labNodes[node.Name] = node.Image
	}

	list, err := r.client.Resource(imageRequestGVR).Namespace(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	requests := make([]imagePullRequest, 0, len(list.Items))
	for idx := range list.Items {
		request := imagePullRequestFromResource(&list.Items[idx])
		expectedImage, belongsToLab := labNodes[request.node]
		if !belongsToLab || request.image != expectedImage {
			continue
		}
		requests = append(requests, request)
	}
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].name < requests[j].name
	})

	return requests, nil
}

func imagePullRequestFromResource(resource *unstructured.Unstructured) imagePullRequest {
	request := imagePullRequest{name: resource.GetName()}
	request.node, _, _ = unstructured.NestedString(
		resource.Object, "spec", "topologyNodeName",
	)
	request.image, _, _ = unstructured.NestedString(
		resource.Object, "spec", "requestedImage",
	)
	request.kubernetesNode, _, _ = unstructured.NestedString(
		resource.Object, "spec", "kubernetesNode",
	)
	request.complete, _, _ = unstructured.NestedBool(
		resource.Object, "status", "complete",
	)

	return request
}

func (p *imagePullProgress) report(requests []imagePullRequest) {
	p.listErrorReported = false
	next := make(map[string]imagePullRequest, len(requests))
	for _, request := range requests {
		previous, seen := p.active[request.name]
		next[request.name] = request
		if !seen {
			logImagePullProgress("Pulling clabernetes node image", request)
		}
		if request.complete && (!seen || !previous.complete) {
			logImagePullProgress("Clabernetes node image pull completed", request)
		}
	}

	for name, request := range p.active {
		if _, stillActive := next[name]; stillActive || request.complete {
			continue
		}
		// The controller deletes an ImageRequest after its puller leaves Pending. Usually that
		// deletion is observed between polls, before the brief complete status can be listed.
		logImagePullProgress("Clabernetes node image pull completed", request)
	}

	p.active = next
}

func (p *imagePullProgress) reportListError(err error) {
	if p.listErrorReported {
		return
	}
	log.Debug("Unable to inspect clabernetes image pulls", "error", err)
	p.listErrorReported = true
}

func (p *imagePullProgress) inspectLauncherCopies(
	ctx context.Context,
	r *Runtime,
	namespace string,
	state *clablabruntime.LabState,
) {
	labNodes := make(map[string]string, len(state.Nodes))
	for _, node := range state.Nodes {
		labNodes[node.Name] = node.Image
	}

	pods, err := r.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: state.Name,
		}.String(),
	})
	if err != nil {
		if !p.launcherListErrorReported {
			log.Debug("Unable to inspect clabernetes launcher pods", "error", err)
			p.launcherListErrorReported = true
		}

		return
	}
	p.launcherListErrorReported = false
	sort.Slice(pods.Items, func(i, j int) bool {
		return pods.Items[i].Name < pods.Items[j].Name
	})

	for idx := range pods.Items {
		pod := &pods.Items[idx]
		nodeName := pod.Labels[labelTopologyNode]
		image, belongsToLab := labNodes[nodeName]
		if !belongsToLab || image == "" || pod.Status.Phase != corev1.PodRunning {
			continue
		}

		key := string(pod.UID)
		if key == "" {
			key = pod.Name
		}
		if p.launcherCopies[key].complete {
			continue
		}

		content, err := r.kubeClient.CoreV1().Pods(namespace).GetLogs(
			pod.Name,
			&corev1.PodLogOptions{TailLines: pointerTo(launcherLogTailLines)},
		).DoRaw(ctx)
		if err != nil {
			p.reportLauncherLogError(key, pod.Name, err)
			continue
		}
		delete(p.launcherLogErrors, key)
		p.reportLauncherLog(launcherImageLog{
			podName:        key,
			node:           nodeName,
			image:          image,
			kubernetesNode: pod.Spec.NodeName,
			content:        string(content),
		})
	}
}

func (p *imagePullProgress) reportLauncherLog(observation launcherImageLog) {
	progress := p.launcherCopies[observation.podName]
	fields := imagePullRequest{
		node:           observation.node,
		image:          observation.image,
		kubernetesNode: observation.kubernetesNode,
	}

	imageQuoted := `image "` + observation.image + `"`
	if !progress.copying && strings.Contains(
		observation.content,
		imageQuoted+" is present, begin copy to docker daemon",
	) {
		logImagePullProgress(
			"Clabernetes node image already present on Kubernetes node; "+
				"copying to launcher Docker daemon",
			fields,
		)
		progress.copying = true
	}

	if !progress.copying && strings.Contains(
		observation.content,
		imageQuoted+" is now available on node, continuing",
	) {
		logImagePullProgress(
			"Copying clabernetes node image to launcher Docker daemon",
			fields,
		)
		progress.copying = true
	}

	if !progress.complete && strings.Contains(
		observation.content,
		"Loaded image: "+observation.image,
	) {
		progress.copying = true
		progress.complete = true
	}

	if p.launcherCopies == nil {
		p.launcherCopies = map[string]launcherImageCopyProgress{}
	}
	p.launcherCopies[observation.podName] = progress
}

func (p *imagePullProgress) reportLauncherLogError(key, podName string, err error) {
	if _, reported := p.launcherLogErrors[key]; reported {
		return
	}
	log.Debug("Unable to inspect clabernetes launcher logs", "pod", podName, "error", err)
	if p.launcherLogErrors == nil {
		p.launcherLogErrors = map[string]struct{}{}
	}
	p.launcherLogErrors[key] = struct{}{}
}

func pointerTo[T any](value T) *T {
	return &value
}

func logImagePullProgress(message string, request imagePullRequest) {
	log.Info(
		message,
		"node", request.node,
		"image", request.image,
		"kubernetes-node", request.kubernetesNode,
	)
}
