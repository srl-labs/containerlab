// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	border0 "github.com/srl-labs/containerlab/nodes/border0"
	bridge "github.com/srl-labs/containerlab/nodes/bridge"
	c8000 "github.com/srl-labs/containerlab/nodes/c8000"
	ceos "github.com/srl-labs/containerlab/nodes/ceos"
	checkpoint_cloudguard "github.com/srl-labs/containerlab/nodes/checkpoint_cloudguard"
	crpd "github.com/srl-labs/containerlab/nodes/crpd"
	cvx "github.com/srl-labs/containerlab/nodes/cvx"
	"github.com/srl-labs/containerlab/nodes/dell_sonic"
	ext_container "github.com/srl-labs/containerlab/nodes/ext_container"
	fortinet_fortigate "github.com/srl-labs/containerlab/nodes/fortinet_fortigate"
	generic_vm "github.com/srl-labs/containerlab/nodes/generic_vm"
	host "github.com/srl-labs/containerlab/nodes/host"
	ipinfusion_ocnos "github.com/srl-labs/containerlab/nodes/ipinfusion_ocnos"
	k8s_kind "github.com/srl-labs/containerlab/nodes/k8s_kind"
	keysight_ixiacone "github.com/srl-labs/containerlab/nodes/keysight_ixiacone"
	linux "github.com/srl-labs/containerlab/nodes/linux"
	ovs "github.com/srl-labs/containerlab/nodes/ovs"
	rare "github.com/srl-labs/containerlab/nodes/rare"
	sonic "github.com/srl-labs/containerlab/nodes/sonic"
	vr_sonic "github.com/srl-labs/containerlab/nodes/sonic_vm"
	srl "github.com/srl-labs/containerlab/nodes/srl"
	vr_aoscx "github.com/srl-labs/containerlab/nodes/vr_aoscx"
	vr_c8000v "github.com/srl-labs/containerlab/nodes/vr_c8000v"
	vr_cat9kv "github.com/srl-labs/containerlab/nodes/vr_cat9kv"
	vr_csr "github.com/srl-labs/containerlab/nodes/vr_csr"
	vr_freebsd "github.com/srl-labs/containerlab/nodes/vr_freebsd"
	vr_ftdv "github.com/srl-labs/containerlab/nodes/vr_ftdv"
	vr_ftosv "github.com/srl-labs/containerlab/nodes/vr_ftosv"
	vr_n9kv "github.com/srl-labs/containerlab/nodes/vr_n9kv"
	vr_openbsd "github.com/srl-labs/containerlab/nodes/vr_openbsd"
	vr_pan "github.com/srl-labs/containerlab/nodes/vr_pan"
	vr_ros "github.com/srl-labs/containerlab/nodes/vr_ros"
	vr_sros "github.com/srl-labs/containerlab/nodes/vr_sros"
	vr_veos "github.com/srl-labs/containerlab/nodes/vr_veos"
	vr_vjunosevolved "github.com/srl-labs/containerlab/nodes/vr_vjunosevolved"
	vr_vjunosswitch "github.com/srl-labs/containerlab/nodes/vr_vjunosswitch"
	vr_vmx "github.com/srl-labs/containerlab/nodes/vr_vmx"
	vr_vqfx "github.com/srl-labs/containerlab/nodes/vr_vqfx"
	vr_vsrx "github.com/srl-labs/containerlab/nodes/vr_vsrx"
	vr_xrv "github.com/srl-labs/containerlab/nodes/vr_xrv"
	vr_xrv9k "github.com/srl-labs/containerlab/nodes/vr_xrv9k"
	xrd "github.com/srl-labs/containerlab/nodes/xrd"
)

// RegisterNodes registers all the nodes/kinds supported by containerlab.
func (c *CLab) RegisterNodes() {
	bridge.Register(c.Reg)
	ceos.Register(c.Reg)
	checkpoint_cloudguard.Register(c.Reg)
	crpd.Register(c.Reg)
	cvx.Register(c.Reg)
	ext_container.Register(c.Reg)
	fortinet_fortigate.Register(c.Reg)
	host.Register(c.Reg)
	ipinfusion_ocnos.Register(c.Reg)
	keysight_ixiacone.Register(c.Reg)
	linux.Register(c.Reg)
	ovs.Register(c.Reg)
	sonic.Register(c.Reg)
	srl.Register(c.Reg)
	vr_aoscx.Register(c.Reg)
	vr_csr.Register(c.Reg)
	vr_c8000v.Register(c.Reg)
	vr_freebsd.Register(c.Reg)
	generic_vm.Register(c.Reg)
	dell_sonic.Register(c.Reg)
	vr_ftosv.Register(c.Reg)
	vr_n9kv.Register(c.Reg)
	vr_pan.Register(c.Reg)
	vr_openbsd.Register(c.Reg)
	vr_ftdv.Register(c.Reg)
	vr_ros.Register(c.Reg)
	vr_sros.Register(c.Reg)
	vr_veos.Register(c.Reg)
	vr_vmx.Register(c.Reg)
	vr_vsrx.Register(c.Reg)
	vr_vqfx.Register(c.Reg)
	vr_vjunosswitch.Register(c.Reg)
	vr_vjunosevolved.Register(c.Reg)
	vr_xrv.Register(c.Reg)
	vr_xrv9k.Register(c.Reg)
	vr_sonic.Register(c.Reg)
	vr_cat9kv.Register(c.Reg)
	xrd.Register(c.Reg)
	rare.Register(c.Reg)
	c8000.Register(c.Reg)
	border0.Register(c.Reg)
	k8s_kind.Register(c.Reg)
}
