// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package k8s_kind

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime/docker"
	"github.com/srl-labs/containerlab/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
)

var kindnames = []string{"k8s-kind"}

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(k8s_kind)
	})
}

type k8s_kind struct {
	nodes.DefaultNode
}

func (k *k8s_kind) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	k.DefaultNode = *nodes.NewDefaultNode(k)
	k.Cfg = cfg
	for _, o := range opts {
		o(k)
	}

	return nil
}

// GetImages is not required, kind will download the images itself
func (d *k8s_kind) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (d *k8s_kind) PullImage(_ context.Context) error             { return nil }

// DeleteNetnsSymlink kind takes care of the Netlinks itself
func (d *k8s_kind) DeleteNetnsSymlink() (err error) { return nil }

func (k *k8s_kind) Deploy(_ context.Context) error {

	// create the Provider with the above runtime based options
	kindProvider, err := k.getProvider()
	if err != nil {
		return err
	}

	// prepare the slice of cluster create options
	clusterCreateOptions := []cluster.CreateOption{}

	// set the kind image, if provided
	if k.Cfg.Image != "" {
		clusterCreateOptions = append(clusterCreateOptions, cluster.CreateWithNodeImage(k.Cfg.Image))
	}

	// Read the kind cluster config
	conf, err := readClusterConfig(k.Cfg.StartupConfig)
	if err != nil {
		return err
	}

	// set the byteConfig as the config to use
	clusterCreateOptions = append(clusterCreateOptions, cluster.CreateWithV1Alpha4Config(conf))
	// make the Create call synchronous, but use a timeout of 15 min.
	clusterCreateOptions = append(clusterCreateOptions, cluster.CreateWithWaitForReady(time.Duration(15)*time.Minute))

	// create the kind cluster
	err = kindProvider.Create(k.Cfg.ShortName, clusterCreateOptions...)
	if err != nil {
		return err
	}

	return err
}

func (k *k8s_kind) GetRuntimeInformation(ctx context.Context) ([]types.GenericContainer, error) {
	containeList, err := k.Runtime.ListContainers(ctx, []*types.GenericFilter{
		{
			FilterType: "label",
			Field:      "io.x-k8s.kind.cluster",
			Operator:   "=",
			Match:      k.Cfg.ShortName, // this regexp ensure we have an exact match for name
		},
	})
	if err != nil {
		return nil, err
	}
	for _, cnt := range containeList {
		// fake fill the returned labels with the configured once.
		// Some of the displayed information is read from labels (e.g Kind)
		for key, v := range k.Cfg.Labels {
			cnt.Labels[key] = v
		}
		// we need to overwrite the nodename label
		cnt.Labels["clab-node-name"] = cnt.Names[0]
	}
	return containeList, nil
}

func (k *k8s_kind) Delete(_ context.Context) error {
	// create the Provider with the above runtime based options
	kindProvider, err := k.getProvider()
	if err != nil {
		return err
	}
	log.Infof("Deleting kind cluster %q", k.Cfg.ShortName)
	return kindProvider.Delete(k.Cfg.ShortName, "")
}

// getProvider returns the kind provider (runtime for kind)
func (k *k8s_kind) getProvider() (*cluster.Provider, error) {
	var kindProviderOptions cluster.ProviderOption
	// instantiate the Provider which is runtime dependent
	switch k.Runtime.GetName() {
	case docker.RuntimeName:
		kindProviderOptions = cluster.ProviderWithDocker()
	default:
		return nil, fmt.Errorf("runtime %s not supported by the k8s_kind node kind", k.Runtime.GetName())
	}

	// create the Provider with the above runtime based options
	return cluster.NewProvider(kindProviderOptions), nil
}

// readClusterConfig reads the kind clusterconfig from a file and returns the
// parsed struct
func readClusterConfig(configfile string) (*v1alpha4.Cluster, error) {

	// unmarshal the clusterconfig
	clusterConfig := &v1alpha4.Cluster{}

	if configfile != "" {
		// open and read the file
		data, err := ioutil.ReadFile(configfile)
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
