// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package k8s_kind

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/runtime/docker"
	"github.com/srl-labs/containerlab/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
)

var kindnames = []string{"k8s-kind"}

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
		return new(k8s_kind)
	}, nil)
}

type k8s_kind struct {
	nodes.DefaultNode
}

func (n *k8s_kind) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	n.DefaultNode = *nodes.NewDefaultNode(n)
	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	return nil
}

// GetImages is not required, kind will download the images.
func (n *k8s_kind) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (n *k8s_kind) PullImage(_ context.Context) error             { return nil }

// DeleteNetnsSymlink is a noop since kind takes care of the Netlinks.
func (n *k8s_kind) DeleteNetnsSymlink() (err error) { return nil }

func (n *k8s_kind) Deploy(_ context.Context, _ *nodes.DeployParams) error {
	// create the Provider with the above runtime based options
	kindProvider, err := n.getProvider()
	if err != nil {
		return err
	}

	// prepare the slice of cluster create options
	clusterCreateOptions := []cluster.CreateOption{}

	// set the kind image, if provided
	if n.Cfg.Image != "" {
		clusterCreateOptions = append(clusterCreateOptions,
			cluster.CreateWithNodeImage(n.Cfg.Image))
	}

	// Read the kind cluster config
	conf, err := readClusterConfig(n.Cfg.StartupConfig)
	if err != nil {
		return err
	}

	clusterCreateOptions = append(clusterCreateOptions,
		// set the byteConfig as the config to use
		cluster.CreateWithV1Alpha4Config(conf),
		// make the Create call synchronous, but use a timeout of 15 min.
		// This may be overridden by the user in the extra config.
		cluster.CreateWithWaitForReady(time.Duration(15)*time.Minute),
	)

	// Handle extra deploy options
	if n.Cfg.Extras != nil && n.Cfg.Extras.K8sKind != nil &&
		n.Cfg.Extras.K8sKind.Deploy != nil {
		opts := n.Cfg.Extras.K8sKind.Deploy

		// Override the default wait duration
		if opts.Wait != nil {
			duration, err := time.ParseDuration(*opts.Wait)
			if err != nil {
				return fmt.Errorf("failed to parse wait duration: %w", err)
			}
			clusterCreateOptions = append(clusterCreateOptions,
				cluster.CreateWithWaitForReady(duration))
		}
	}

	// create the kind cluster
	err = kindProvider.Create(n.Cfg.ShortName, clusterCreateOptions...)
	if err != nil {
		return err
	}

	return err
}

func (n *k8s_kind) GetContainers(ctx context.Context) ([]runtime.GenericContainer, error) {
	containeList, err := n.Runtime.ListContainers(ctx, []*types.GenericFilter{
		{
			FilterType: "label",
			Field:      "io.x-k8s.kind.cluster",
			Operator:   "=",
			Match:      n.Cfg.ShortName, // this regexp ensures we have an exact match for name
		},
	})
	if err != nil {
		return nil, err
	}
	for _, cnt := range containeList {
		// fake fill the returned labels with the configured once.
		// Some of the displayed information is read from labels (e.g Kind)
		for key, v := range n.Cfg.Labels {
			cnt.Labels[key] = v
		}
		// we need to overwrite the nodename label
		cnt.Labels["clab-node-name"] = cnt.Names[0]
	}

	return containeList, nil
}

func (n *k8s_kind) Delete(_ context.Context) error {
	// create the Provider with the above runtime based options
	kindProvider, err := n.getProvider()
	if err != nil {
		return err
	}
	log.Infof("Deleting kind cluster %q", n.Cfg.ShortName)

	return kindProvider.Delete(n.Cfg.ShortName, "")
}

// getProvider returns the kind provider (runtime for kind).
func (n *k8s_kind) getProvider() (*cluster.Provider, error) {
	var kindProviderOptions cluster.ProviderOption
	// instantiate the Provider which is runtime dependent
	switch n.Runtime.GetName() {
	case docker.RuntimeName:
		kindProviderOptions = cluster.ProviderWithDocker()
	case "podman": // this is an ugly workaround because podman is generally excluded via golang tags ... should be "podman.RuntimeName"
		kindProviderOptions = cluster.ProviderWithPodman()
	default:
		return nil, fmt.Errorf("runtime %s not supported by the k8s_kind node kind", n.Runtime.GetName())
	}

	// create the Provider with the above runtime based options
	return cluster.NewProvider(
		kindProviderOptions,
		cluster.ProviderWithLogger(newKindLogger(n.Cfg.ShortName, 0)),
	), nil
}

// readClusterConfig reads the kind clusterconfig from a file.
func readClusterConfig(configfile string) (*v1alpha4.Cluster, error) {
	// unmarshal the clusterconfig
	clusterConfig := &v1alpha4.Cluster{}

	if configfile != "" {
		// open and read the file
		data, err := os.ReadFile(configfile)
		if err != nil {
			return nil, err
		}
		// unmarshal the date into a kind clusterConfig
		err = yaml.Unmarshal(data, clusterConfig)
		if err != nil {
			return nil, err
		}
	} else {
		clusterConfig.TypeMeta = v1alpha4.TypeMeta{
			APIVersion: "kind.x-k8s.io/v1alpha4",
			Kind:       "Cluster",
		}
		// if no config was provided, generate the default
		v1alpha4.SetDefaultsCluster(clusterConfig)
	}

	return clusterConfig, nil
}

// RunExec is not implemented for this kind.
func (n *k8s_kind) RunExec(_ context.Context, _ *exec.ExecCmd) (*exec.ExecResult, error) {
	log.Warnf("Exec operation is not implemented for kind %q", n.Config().Kind)

	return nil, exec.ErrRunExecNotSupported
}

// RunExecNotWait is not implemented for this kind.
func (n *k8s_kind) RunExecNotWait(_ context.Context, _ *exec.ExecCmd) error {
	log.Warnf("RunExecNotWait operation is not implemented for kind %q", n.Config().Kind)

	return exec.ErrRunExecNotSupported
}
