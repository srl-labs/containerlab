// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package all

import (
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
func RegisterAll() {
	bridge.Register()
	ceos.Register()
	checkpoint_cloudguard.Register()
	crpd.Register()
	cvx.Register()
	ext_container.Register()
	host.Register()
	ipinfusion_ocnos.Register()
	keysight_ixiacone.Register()
	linux.Register()
	mysocketio.Register()
	ovs.Register()
	sonic.Register()
	srl.Register()
	vr_csr.Register()
	vr_ftosv.Register()
	vr_n9kv.Register()
	vr_nxos.Register()
	vr_pan.Register()
	vr_ros.Register()
	vr_sros.Register()
	vr_veos.Register()
	vr_vmx.Register()
	vr_vqfx.Register()
	vr_xrv.Register()
	vr_xrv9k.Register()
	xrd.Register()
}
