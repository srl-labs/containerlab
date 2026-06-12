package clabernetes

import (
	"context"
	"fmt"
	"sort"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (r *Runtime) Start(ctx context.Context, req clablabruntime.NodeRequest) error {
	return r.setNodesReplicas(ctx, req, 1)
}

func (r *Runtime) Stop(ctx context.Context, req clablabruntime.NodeRequest) error {
	if err := r.setTopologyIgnoreReconcile(ctx, req.Name, req.Namespace, true); err != nil {
		return err
	}

	return r.setNodesReplicas(ctx, req, 0)
}

func (r *Runtime) Restart(ctx context.Context, req clablabruntime.NodeRequest) error {
	targets, namespace, err := r.targetNodes(ctx, req)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, nodeName := range targets {
		deployment, err := r.deploymentForNode(ctx, req.Name, namespace, nodeName)
		if err != nil {
			return err
		}

		if deployment.Spec.Template.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
		}
		deployment.Spec.Template.ObjectMeta.Annotations[restartedAtAnnotation] = now

		if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
			replicas := int32(1)
			deployment.Spec.Replicas = &replicas
		}

		_, err = r.kubeClient.AppsV1().Deployments(namespace).
			Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to restart clabernetes node %s/%s/%s: %w",
				namespace, req.Name, nodeName, err)
		}

		if err := r.waitDeploymentReplicas(ctx, namespace, deployment.Name, 1, req.Timeout); err != nil {
			return err
		}
	}

	return r.clearIgnoreWhenAllStarted(ctx, req.Name, namespace)
}

func (r *Runtime) targetNodes(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) ([]string, string, error) {
	if req.Name == "" {
		return nil, "", fmt.Errorf("topology name is required")
	}

	namespace := r.namespaceFor(req.Namespace)
	deployments, err := r.deploymentsForTopology(ctx, req.Name, namespace)
	if err != nil {
		return nil, "", err
	}

	known := map[string]struct{}{}
	for idx := range deployments.Items {
		nodeName := deployments.Items[idx].Labels[labelTopologyNode]
		if nodeName != "" {
			known[nodeName] = struct{}{}
		}
	}

	if len(known) == 0 {
		state, err := r.Inspect(ctx, clablabruntime.InspectRequest{Name: req.Name, Namespace: namespace})
		if err != nil {
			return nil, "", err
		}
		for _, node := range state.Nodes {
			known[node.Name] = struct{}{}
		}
	}

	if len(known) == 0 {
		return nil, "", fmt.Errorf("topology %s/%s has no nodes", namespace, req.Name)
	}

	var targets []string
	if len(req.Nodes) == 0 {
		targets = make([]string, 0, len(known))
		for nodeName := range known {
			targets = append(targets, nodeName)
		}
		sort.Strings(targets)

		return targets, namespace, nil
	}

	for _, nodeName := range req.Nodes {
		if _, ok := known[nodeName]; !ok {
			return nil, "", fmt.Errorf("node %q was not found in topology %s/%s",
				nodeName, namespace, req.Name)
		}
		targets = append(targets, nodeName)
	}

	return targets, namespace, nil
}

func (r *Runtime) setNodesReplicas(
	ctx context.Context,
	req clablabruntime.NodeRequest,
	replicas int32,
) error {
	targets, namespace, err := r.targetNodes(ctx, req)
	if err != nil {
		return err
	}

	for _, nodeName := range targets {
		deployment, err := r.deploymentForNode(ctx, req.Name, namespace, nodeName)
		if err != nil {
			return err
		}

		deployment.Spec.Replicas = &replicas
		_, err = r.kubeClient.AppsV1().Deployments(namespace).
			Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to set clabernetes node %s/%s/%s replicas to %d: %w",
				namespace, req.Name, nodeName, replicas, err)
		}

		if err := r.waitDeploymentReplicas(ctx, namespace, deployment.Name, replicas, req.Timeout); err != nil {
			return err
		}
	}

	if replicas > 0 {
		return r.clearIgnoreWhenAllStarted(ctx, req.Name, namespace)
	}

	return nil
}

func (r *Runtime) clearIgnoreWhenAllStarted(ctx context.Context, name, namespace string) error {
	deployments, err := r.deploymentsForTopology(ctx, name, namespace)
	if err != nil {
		return err
	}

	for idx := range deployments.Items {
		replicas := int32(1)
		if deployments.Items[idx].Spec.Replicas != nil {
			replicas = *deployments.Items[idx].Spec.Replicas
		}
		if replicas == 0 {
			return nil
		}
	}

	return r.setTopologyIgnoreReconcile(ctx, name, namespace, false)
}

func (r *Runtime) setTopologyIgnoreReconcile(
	ctx context.Context,
	name,
	namespace string,
	enabled bool,
) error {
	if name == "" {
		return fmt.Errorf("topology name is required")
	}

	namespace = r.namespaceFor(namespace)
	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	obj, err := resource.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get clabernetes topology %s/%s: %w",
			namespace, name, err)
	}

	labelsMap := obj.GetLabels()
	if labelsMap == nil {
		labelsMap = map[string]string{}
	}

	if enabled {
		labelsMap[labelIgnoreReconcile] = "true"
	} else {
		delete(labelsMap, labelIgnoreReconcile)
	}

	obj.SetLabels(labelsMap)

	_, err = resource.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update clabernetes topology %s/%s labels: %w",
			namespace, name, err)
	}

	return nil
}

func (r *Runtime) waitDeploymentReplicas(
	ctx context.Context,
	namespace,
	name string,
	replicas int32,
	timeout time.Duration,
) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(timeout))
	defer cancel()

	return wait.PollUntilContextCancel(waitCtx, pollInterval, true,
		func(ctx context.Context) (bool, error) {
			deployment, err := r.kubeClient.AppsV1().Deployments(namespace).
				Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to get clabernetes deployment %s/%s: %w",
					namespace, name, err)
			}

			if replicas == 0 {
				return deployment.Status.Replicas == 0 &&
					deployment.Status.AvailableReplicas == 0, nil
			}

			return deployment.Status.ReadyReplicas >= replicas &&
				deployment.Status.AvailableReplicas >= replicas, nil
		})
}

func (r *Runtime) deploymentsForTopology(
	ctx context.Context,
	name,
	namespace string,
) (*appsv1.DeploymentList, error) {
	namespace = r.namespaceFor(namespace)
	list, err := r.kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: name,
		}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes deployments for topology %s/%s: %w",
			namespace, name, err)
	}

	return list, nil
}

func (r *Runtime) deploymentForNode(
	ctx context.Context,
	name,
	namespace,
	nodeName string,
) (*appsv1.Deployment, error) {
	namespace = r.namespaceFor(namespace)
	list, err := r.kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: name,
			labelTopologyNode:  nodeName,
		}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list clabernetes deployment for node %s/%s/%s: %w",
			namespace, name, nodeName, err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("clabernetes deployment for node %s/%s/%s was not found",
			namespace, name, nodeName)
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf(
			"expected exactly one clabernetes deployment for node %s/%s/%s, found %d",
			namespace, name, nodeName, len(list.Items))
	}

	return &list.Items[0], nil
}
