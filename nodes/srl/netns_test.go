package srl

import (
	"context"
	"testing"

	"github.com/docker/go-connections/nat"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testNodeName   = "clab-test-srl1"
	testNetnsName  = "clab-test-srl1-netns"
	testNetnsShort = "srl1-netns"
)

func newSRLTestConfig(t *testing.T) *clabtypes.NodeConfig {
	t.Helper()

	return &clabtypes.NodeConfig{
		ShortName:   "srl1",
		LongName:    testNodeName,
		Kind:        srlLongPlatformName,
		Image:       "srlinux:test",
		LabDir:      t.TempDir(),
		Env:         map[string]string{"FOO": "bar"},
		Sysctls:     map[string]string{},
		Labels:      map[string]string{clabconstants.Containerlab: "test"},
		Certificate: &clabtypes.CertificateConfig{},
	}
}

func newSRLTestNode(
	t *testing.T,
	cfg *clabtypes.NodeConfig,
	rt clabruntime.ContainerRuntime,
) *srl {
	t.Helper()

	n := &srl{}
	require.NoError(t, n.Init(cfg, clabnodes.WithRuntime(rt)))

	return n
}

func TestNetnsNodeConfig(t *testing.T) {
	cfg := newSRLTestConfig(t)
	cfg.MgmtIPv4Address = "172.20.20.10"
	n := newSRLTestNode(t, cfg, nil)

	require.NotNil(t, n.netnsNode)
	holder := n.netnsNode.Config()

	assert.Equal(t, "unless-stopped", n.Cfg.RestartPolicy)
	assert.Equal(t, testNetnsShort, holder.ShortName)
	assert.Equal(t, testNetnsName, holder.LongName)
	assert.Equal(t, n.Cfg.ShortName, holder.GetHostname())
	assert.Equal(t, n.Cfg.Image, holder.Image)
	assert.Equal(t, n.Cfg.RestartPolicy, holder.RestartPolicy)
	assert.Equal(t, n.Cfg.MgmtIPv4Address, holder.MgmtIPv4Address)
	assert.Equal(t, n.Cfg.Sysctls, holder.Sysctls)
	assert.Equal(t, "/bin/sh", holder.Entrypoint)
	assert.Empty(t, holder.Env)
	assert.Empty(t, holder.Binds)
	assert.Equal(t, "true", holder.Labels[clabconstants.InternalNode])
	assert.Equal(t, netnsNodeType, holder.Labels[clabconstants.NodeType])
	assert.Equal(t, n.Cfg.ShortName, holder.Labels[clabconstants.RootNodeName])
	assert.Equal(t, n.Cfg.LongName, holder.Labels[clabconstants.RootNodeLongName])
	assert.Empty(t, n.Cfg.Labels[clabconstants.InternalNode])
}

func TestUserRestartPolicyIsKept(t *testing.T) {
	cfg := newSRLTestConfig(t)
	cfg.RestartPolicy = "no"
	n := newSRLTestNode(t, cfg, nil)

	assert.Equal(t, "no", n.Cfg.RestartPolicy)
	assert.Equal(t, "no", n.netnsNode.Config().RestartPolicy)
}

func TestUserNetworkModeSkipsNetnsNode(t *testing.T) {
	cfg := newSRLTestConfig(t)
	cfg.NetworkMode = "host"
	n := newSRLTestNode(t, cfg, nil)

	assert.Nil(t, n.netnsNode)
	assert.Equal(t, "unless-stopped", n.Cfg.RestartPolicy)
}

func TestContainerConfigUsesNetnsNode(t *testing.T) {
	cfg := newSRLTestConfig(t)
	cfg.DNS = &clabtypes.DNSConfig{Servers: []string{"1.1.1.1"}}
	cfg.ExtraHosts = []string{"srl2:172.20.20.3"}
	cfg.MacAddress = "02:00:00:00:00:01"
	cfg.PortSet = nat.PortSet{"443/tcp": struct{}{}}
	cfg.PortBindings = nat.PortMap{"443/tcp": []nat.PortBinding{{HostPort: "50443"}}}
	n := newSRLTestNode(t, cfg, nil)

	got := n.containerConfig()

	assert.Equal(t, "container:"+testNetnsShort, got.NetworkMode)
	assert.Nil(t, got.DNS)
	assert.Nil(t, got.ExtraHosts)
	assert.Empty(t, got.MacAddress)
	assert.Nil(t, got.PortBindings)
	assert.Nil(t, got.PortSet)
	assert.Equal(t, n.Cfg.Entrypoint, got.Entrypoint)
	assert.Equal(t, n.Cfg.Cmd, got.Cmd)
	assert.Equal(t, "bar", got.Env["FOO"])

	// the node config keeps the network settings, they are applied to the holder
	assert.Empty(t, n.Cfg.NetworkMode)
	assert.Equal(t, []string{"srl2:172.20.20.3"}, n.Cfg.ExtraHosts)
	assert.NotNil(t, n.Cfg.DNS)
}

func TestGetContainersUsesNetnsNodeNetwork(t *testing.T) {
	ctx := context.Background()
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n := newSRLTestNode(t, newSRLTestConfig(t), rt)

	ports := []*clabtypes.GenericPortBinding{{HostPort: 50443, ContainerPort: 443}}
	holder := clabruntime.GenericContainer{
		Names:       []string{testNetnsName},
		NetworkName: "clab",
		NetworkSettings: clabruntime.GenericMgmtIPs{
			IPv4addr: "172.20.20.10",
			IPv4pLen: 24,
			IPv6addr: "3fff:172:20:20::10",
			IPv6pLen: 64,
		},
		Ports: ports,
	}

	rt.EXPECT().ListContainers(ctx, []*clabtypes.GenericFilter{
		{FilterType: "name", Match: testNodeName},
	}).Return([]clabruntime.GenericContainer{{Names: []string{testNodeName}, ID: "srl"}}, nil)
	rt.EXPECT().ListContainers(ctx, []*clabtypes.GenericFilter{
		{FilterType: "name", Match: testNetnsName},
	}).Return([]clabruntime.GenericContainer{holder}, nil)

	got, err := n.GetContainers(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "srl", got[0].ID)
	assert.Equal(t, holder.NetworkName, got[0].NetworkName)
	assert.Equal(t, holder.NetworkSettings, got[0].NetworkSettings)
	assert.Equal(t, ports, got[0].Ports)
}

func TestGetContainersReturnsNetnsNodeWhenNodeIsMissing(t *testing.T) {
	ctx := context.Background()
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n := newSRLTestNode(t, newSRLTestConfig(t), rt)

	holder := clabruntime.GenericContainer{Names: []string{testNetnsName}}
	rt.EXPECT().ListContainers(ctx, []*clabtypes.GenericFilter{
		{FilterType: "name", Match: testNodeName},
	}).Return(nil, nil)
	rt.EXPECT().ListContainers(ctx, []*clabtypes.GenericFilter{
		{FilterType: "name", Match: testNetnsName},
	}).Return([]clabruntime.GenericContainer{holder}, nil)

	got, err := n.GetContainers(ctx)
	require.NoError(t, err)
	assert.Equal(t, []clabruntime.GenericContainer{holder}, got)
}

func TestDeleteRemovesNetnsNode(t *testing.T) {
	ctx := context.Background()
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n := newSRLTestNode(t, newSRLTestConfig(t), rt)

	gomock.InOrder(
		rt.EXPECT().GetContainerStatus(ctx, testNodeName).Return(clabruntime.Running),
		rt.EXPECT().DeleteContainer(ctx, testNodeName).Return(nil),
		rt.EXPECT().GetContainerStatus(ctx, testNetnsName).Return(clabruntime.Running),
		rt.EXPECT().DeleteContainer(ctx, testNetnsName).Return(nil),
	)

	require.NoError(t, n.Delete(ctx))
}

func TestStopStopsNetnsNode(t *testing.T) {
	ctx := context.Background()

	t.Run("running holder is stopped", func(t *testing.T) {
		rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
		n := newSRLTestNode(t, newSRLTestConfig(t), rt)

		rt.EXPECT().GetContainerStatus(ctx, testNodeName).Return(clabruntime.Stopped)
		rt.EXPECT().GetContainerStatus(ctx, testNetnsName).Return(clabruntime.Running)
		rt.EXPECT().StopContainer(ctx, testNetnsName, clabtypes.SIGTERM).Return(nil)

		require.NoError(t, n.Stop(ctx))
	})

	t.Run("stopped holder is left alone", func(t *testing.T) {
		rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
		n := newSRLTestNode(t, newSRLTestConfig(t), rt)

		rt.EXPECT().GetContainerStatus(ctx, testNodeName).Return(clabruntime.Stopped)
		rt.EXPECT().GetContainerStatus(ctx, testNetnsName).Return(clabruntime.Stopped)

		require.NoError(t, n.Stop(ctx))
	})
}

func TestStartRequiresNetnsNode(t *testing.T) {
	ctx := context.Background()
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n := newSRLTestNode(t, newSRLTestConfig(t), rt)

	rt.EXPECT().GetContainerStatus(ctx, testNetnsName).Return(clabruntime.NotFound)

	err := n.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recreate the node")
}
