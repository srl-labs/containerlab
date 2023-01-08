// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

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

// RegisterNodes registers all the nodes/kinds supported by containerlab.
func (c *CLab) RegisterNodes() {
	bridge.Register(c.reg)
	ceos.Register(c.reg)
	checkpoint_cloudguard.Register(c.reg)
	crpd.Register(c.reg)
	cvx.Register(c.reg)
	ext_container.Register(c.reg)
	host.Register(c.reg)
	ipinfusion_ocnos.Register(c.reg)
	keysight_ixiacone.Register(c.reg)
	linux.Register(c.reg)
	mysocketio.Register(c.reg)
	ovs.Register(c.reg)
	sonic.Register(c.reg)
	srl.Register(c.reg)
	vr_csr.Register(c.reg)
	vr_ftosv.Register(c.reg)
	vr_n9kv.Register(c.reg)
	vr_nxos.Register(c.reg)
	vr_pan.Register(c.reg)
	vr_ros.Register(c.reg)
	vr_sros.Register(c.reg)
	vr_veos.Register(c.reg)
	vr_vmx.Register(c.reg)
	vr_vqfx.Register(c.reg)
	vr_xrv.Register(c.reg)
	vr_xrv9k.Register(c.reg)
	xrd.Register(c.reg)
}
