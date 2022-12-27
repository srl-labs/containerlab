// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package all

import (
	"github.com/srl-labs/containerlab/nodes"
	bridge "github.com/srl-labs/containerlab/nodes/bridge"
	ceos "github.com/srl-labs/containerlab/nodes/ceos"
	checkpoint_cloudguard "github.com/srl-labs/containerlab/nodes/checkpoint_cloudguard"
	crpd "github.com/srl-labs/containerlab/nodes/crpd"
	cvx "github.com/srl-labs/containerlab/nodes/cvx"
	ext_container "github.com/srl-labs/containerlab/nodes/ext_container"
	host "github.com/srl-labs/containerlab/nodes/host"
	ipinfusion_ocnos "github.com/srl-labs/containerlab/nodes/ipinfusion_ocnos"
	keysight_ixiacone "github.com/srl-labs/containerlab/nodes/keysight_ixiacone"
	linux "github.com/srl-labs/containerlab/nodes/linux"
	mysocketio "github.com/srl-labs/containerlab/nodes/mysocketio"
	ovs "github.com/srl-labs/containerlab/nodes/ovs"
	sonic "github.com/srl-labs/containerlab/nodes/sonic"
	srl "github.com/srl-labs/containerlab/nodes/srl"
	vr_csr "github.com/srl-labs/containerlab/nodes/vr_csr"
	vr_ftosv "github.com/srl-labs/containerlab/nodes/vr_ftosv"
	vr_n9kv "github.com/srl-labs/containerlab/nodes/vr_n9kv"
	vr_nxos "github.com/srl-labs/containerlab/nodes/vr_nxos"
	vr_pan "github.com/srl-labs/containerlab/nodes/vr_pan"
	vr_ros "github.com/srl-labs/containerlab/nodes/vr_ros"
	vr_sros "github.com/srl-labs/containerlab/nodes/vr_sros"
	vr_veos "github.com/srl-labs/containerlab/nodes/vr_veos"
	vr_vmx "github.com/srl-labs/containerlab/nodes/vr_vmx"
	vr_vqfx "github.com/srl-labs/containerlab/nodes/vr_vqfx"
	vr_xrv "github.com/srl-labs/containerlab/nodes/vr_xrv"
	vr_xrv9k "github.com/srl-labs/containerlab/nodes/vr_xrv9k"
	xrd "github.com/srl-labs/containerlab/nodes/xrd"
)

// RegisterAll registers all the nodes/kinds supported by containerlab.
func RegisterAll(r nodes.NodeRergistryRegistrator) {
	bridge.Register(r)
	ceos.Register(r)
	checkpoint_cloudguard.Register(r)
	crpd.Register(r)
	cvx.Register(r)
	ext_container.Register(r)
	host.Register(r)
	ipinfusion_ocnos.Register(r)
	keysight_ixiacone.Register(r)
	linux.Register(r)
	mysocketio.Register(r)
	ovs.Register(r)
	sonic.Register(r)
	srl.Register(r)
	vr_csr.Register(r)
	vr_ftosv.Register(r)
	vr_n9kv.Register(r)
	vr_nxos.Register(r)
	vr_pan.Register(r)
	vr_ros.Register(r)
	vr_sros.Register(r)
	vr_veos.Register(r)
	vr_vmx.Register(r)
	vr_vqfx.Register(r)
	vr_xrv.Register(r)
	vr_xrv9k.Register(r)
	xrd.Register(r)
}
