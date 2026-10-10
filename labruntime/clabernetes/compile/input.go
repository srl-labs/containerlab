// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package compile

import (
	k8scorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Input is the compile contract input: a rendered containerlab topology definition plus the
// deployment policy the clabernetes Topology resource carries. Every field mirrors the
// clabernetes Topology spec subset the compiler consumes; clabernetes converts its typed
// Topology into this input and containerlab's runtime fills it directly.
type Input struct {
	// Name is the clabernetes Topology resource name; it names the shared NodeProfile and
	// labels every emitted object.
	Name string `json:"name"`
	// Namespace is the namespace the primitives are emitted into.
	Namespace string `json:"namespace,omitempty"`
	// Definition is the rendered containerlab topology definition.
	Definition string `json:"definition,omitempty"`
	// Annotations are stamped onto every emitted object as metadata annotations.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels are stamped onto every emitted object as metadata labels, below the compiler's
	// own labels.
	Labels map[string]string `json:"labels,omitempty"`
	// Deployment carries the deployment policy keyed per source node name.
	Deployment Deployment `json:"deployment,omitempty"`
	// DisableManagement disables the c9s management overlay for the topology.
	DisableManagement bool `json:"disableManagement,omitempty"`
	// Expose carries the service exposure policy.
	Expose Expose `json:"expose,omitempty"`
	// ImagePull carries the default image pull policy for the topology.
	ImagePull ImagePull `json:"imagePull,omitempty"`
	// StatusProbes carries the status probe policy; node names follow the definition.
	StatusProbes StatusProbes `json:"statusProbes,omitempty"`
}

// Deployment carries the per-node deployment policy of the clabernetes Topology spec.
type Deployment struct {
	// FilesFromConfigMap maps source node names to ConfigMap-backed file projections.
	FilesFromConfigMap map[string][]FileFromConfigMap `json:"filesFromConfigMap,omitempty"`
	// FilesFromSecret maps source node names to Secret-backed file projections.
	FilesFromSecret map[string][]FileFromSecret `json:"filesFromSecret,omitempty"`
	// FilesFromURL maps source node names to URL-backed file projections.
	FilesFromURL map[string][]FileFromURL `json:"filesFromURL,omitempty"`
	// Resources maps source node names (or DefaultResourceName) to pod resource requirements.
	Resources map[string]*k8scorev1.ResourceRequirements `json:"resources,omitempty"`
	// Scheduling carries the device pod scheduling policy.
	Scheduling *Scheduling `json:"scheduling,omitempty"`
	// Persistence carries the artifact persistence policy.
	Persistence *Persistence `json:"persistence,omitempty"`
}

// Expose carries the service exposure policy of the clabernetes Topology spec.
type Expose struct {
	// DisableAutoExpose disables automatic exposure of the default port list.
	DisableAutoExpose bool `json:"disableAutoExpose,omitempty"`
	// ExposeType configures the Service type used for exposing Nodes.
	ExposeType string `json:"exposeType,omitempty"`
	// UseNodeMgmtIpv4Address assigns a Node's management IPv4 address as its LoadBalancer IP.
	UseNodeMgmtIpv4Address bool `json:"useNodeMgmtIpv4Address,omitempty"`
	// UseNodeMgmtIpv6Address assigns a Node's management IPv6 address as its LoadBalancer IP.
	UseNodeMgmtIpv6Address bool `json:"useNodeMgmtIpv6Address,omitempty"`
}

// ImagePull carries the topology-wide image pull policy.
type ImagePull struct {
	// Policy is the default Kubernetes pull policy for containers whose definition does not
	// explicitly declare one.
	Policy string `json:"policy,omitempty"`
	// PullSecrets provides same-namespace Docker-config Secrets to the kubelet.
	PullSecrets []string `json:"pullSecrets,omitempty"`
}

// StatusProbes carries the status probe policy; node names follow the definition and are
// translated onto the compiled node names by the renderer.
type StatusProbes struct {
	// Enabled sets the status probes to enabled (or obviously disabled).
	Enabled bool `json:"enabled"`
	// ExcludedNodes is a set of definition node names to be excluded from status checking.
	ExcludedNodes []string `json:"excludedNodes,omitempty"`
	// NodeProbeConfigurations maps definition node names to per-node probe configurations.
	NodeProbeConfigurations map[string]ProbeConfiguration `json:"nodeProbeConfigurations,omitempty"`
	// ProbeConfiguration is the default probe configuration for the topology.
	ProbeConfiguration ProbeConfiguration `json:"probeConfiguration,omitempty"`
}

// ProbeConfiguration is one status probe configuration.
type ProbeConfiguration struct {
	// StartupSeconds is the total amount of seconds to allow for the node to start.
	StartupSeconds int `json:"startupSeconds,omitempty"`
	// SSHProbeConfiguration defines an SSH probe.
	SSHProbeConfiguration *SSHProbeConfiguration `json:"sshProbeConfiguration,omitempty"`
	// TCPProbeConfiguration defines a TCP probe.
	TCPProbeConfiguration *TCPProbeConfiguration `json:"tcpProbeConfiguration,omitempty"`
}

// SSHProbeConfiguration defines an SSH status probe.
type SSHProbeConfiguration struct {
	// Username is the username to use for auth.
	Username string `json:"username"`
	// Password is the password to use for auth.
	Password string `json:"password"`
	// Port is an optional override of the default 22.
	Port int `json:"port,omitempty"`
}

// TCPProbeConfiguration defines a TCP status probe.
type TCPProbeConfiguration struct {
	// Port is the port to probe.
	Port int `json:"port,omitempty"`
}

// Scheduling carries the device pod scheduling policy.
type Scheduling struct {
	// NodeSelector sets the node selector configured on all device pods.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations is a list of Tolerations set on the device pod spec.
	Tolerations []k8scorev1.Toleration `json:"tolerations,omitempty"`
	// Affinity sets the affinity rules configured on all direct device Pods.
	Affinity *k8scorev1.Affinity `json:"affinity,omitempty"`
}

// Persistence carries direct device artifact persistence policy.
type Persistence struct {
	// Enabled indicates whether package-planned persistent artifacts are placed in a mounted
	// PVC.
	Enabled bool `json:"enabled"`
	// ClaimSize is the size of the PVC for this topology.
	ClaimSize string `json:"claimSize,omitempty"`
	// StorageClassName is the storage class to set in the PVC.
	StorageClassName string `json:"storageClassName,omitempty"`
	// Reclaim controls the claim's lifetime relative to its Node.
	Reclaim string `json:"reclaim,omitempty"`
}

// FileFromConfigMap is one ConfigMap-backed file projection.
type FileFromConfigMap struct {
	// FilePath is the path to mount the file.
	FilePath string `json:"filePath"`
	// ConfigMapName is the name of the configmap to mount.
	ConfigMapName string `json:"configMapName"`
	// ConfigMapPath is the path/key in the configmap to mount.
	ConfigMapPath string `json:"configMapPath,omitempty"`
	// Mode selects read-only or read-and-execute permissions for the staged file.
	Mode string `json:"mode,omitempty"`
}

// FileFromSecret is one Secret-backed file projection.
type FileFromSecret struct {
	// FilePath is the absolute destination path.
	FilePath string `json:"filePath"`
	// SecretName is the name of the same-namespace Secret.
	SecretName string `json:"secretName"`
	// SecretPath is the Secret data key to project.
	SecretPath string `json:"secretPath,omitempty"`
	// Mode selects read-only or read-and-execute permissions for the staged file.
	Mode string `json:"mode,omitempty"`
}

// FileFromURL is one URL-backed file projection.
type FileFromURL struct {
	// FilePath is the path to mount the file.
	FilePath string `json:"filePath"`
	// URL is the url to fetch and mount at the provided FilePath.
	URL string `json:"url"`
	// Digest is the required SHA-256 identity of the downloaded bytes.
	Digest string `json:"digest,omitempty"`
}

// DefaultResourceName is the Deployment.Resources key for the shared default policy.
const DefaultResourceName = "default"

// renderAll renders the compiled topology into NodeProfiles, Nodes, and Links, every object
// as unstructured and ready for the c9s API server.
func renderAll(
	input *Input,
	compiled *CompiledTopology,
) (nodes, links, nodeProfiles []unstructured.Unstructured, err error) {
	nodeProfiles, err = renderNodeProfiles(input, compiled)
	if err != nil {
		return nil, nil, nil, err
	}

	nodes, err = renderNodes(input, compiled)
	if err != nil {
		return nil, nil, nil, err
	}

	links, err = renderLinks(input, compiled)
	if err != nil {
		return nil, nil, nil, err
	}

	return nodes, links, nodeProfiles, nil
}
