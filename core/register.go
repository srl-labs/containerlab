// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	containerlabnodes6wind_vsr "github.com/srl-labs/containerlab/nodes/6wind_vsr"
	containerlabnodesbridge "github.com/srl-labs/containerlab/nodes/bridge"
	containerlabnodesc8000 "github.com/srl-labs/containerlab/nodes/c8000"
	containerlabnodesceos "github.com/srl-labs/containerlab/nodes/ceos"
	containerlabnodescheckpoint_cloudguard "github.com/srl-labs/containerlab/nodes/checkpoint_cloudguard"
	containerlabnodescjunosevolved "github.com/srl-labs/containerlab/nodes/cjunosevolved"
	containerlabnodescrpd "github.com/srl-labs/containerlab/nodes/crpd"
	containerlabnodescvx "github.com/srl-labs/containerlab/nodes/cvx"
	containerlabnodesdell_sonic "github.com/srl-labs/containerlab/nodes/dell_sonic"
	containerlabnodesext_container "github.com/srl-labs/containerlab/nodes/ext_container"
	containerlabnodesfdio_vpp "github.com/srl-labs/containerlab/nodes/fdio_vpp"
	containerlabnodesfortinet_fortigate "github.com/srl-labs/containerlab/nodes/fortinet_fortigate"
	containerlabnodesgeneric_vm "github.com/srl-labs/containerlab/nodes/generic_vm"
	containerlabnodeshost "github.com/srl-labs/containerlab/nodes/host"
	containerlabnodeshuawei_vrp "github.com/srl-labs/containerlab/nodes/huawei_vrp"
	containerlabnodesiol "github.com/srl-labs/containerlab/nodes/iol"
	containerlabnodesipinfusion_ocnos "github.com/srl-labs/containerlab/nodes/ipinfusion_ocnos"
	containerlabnodesk8s_kind "github.com/srl-labs/containerlab/nodes/k8s_kind"
	containerlabnodeskeysight_ixiacone "github.com/srl-labs/containerlab/nodes/keysight_ixiacone"
	containerlabnodeslinux "github.com/srl-labs/containerlab/nodes/linux"
	containerlabnodesovs "github.com/srl-labs/containerlab/nodes/ovs"
	containerlabnodesrare "github.com/srl-labs/containerlab/nodes/rare"
	containerlabnodessonic "github.com/srl-labs/containerlab/nodes/sonic"
	containerlabnodessonic_vm "github.com/srl-labs/containerlab/nodes/sonic_vm"
	containerlabnodessrl "github.com/srl-labs/containerlab/nodes/srl"
	containerlabnodessros "github.com/srl-labs/containerlab/nodes/sros"
	containerlabnodesvr_aoscx "github.com/srl-labs/containerlab/nodes/vr_aoscx"
	containerlabnodesvr_c8000v "github.com/srl-labs/containerlab/nodes/vr_c8000v"
	containerlabnodesvr_cat9kv "github.com/srl-labs/containerlab/nodes/vr_cat9kv"
	containerlabnodesvr_csr "github.com/srl-labs/containerlab/nodes/vr_csr"
	containerlabnodesvr_freebsd "github.com/srl-labs/containerlab/nodes/vr_freebsd"
	containerlabnodesvr_ftdv "github.com/srl-labs/containerlab/nodes/vr_ftdv"
	containerlabnodesvr_ftosv "github.com/srl-labs/containerlab/nodes/vr_ftosv"
	containerlabnodesvr_n9kv "github.com/srl-labs/containerlab/nodes/vr_n9kv"
	containerlabnodesvr_openbsd "github.com/srl-labs/containerlab/nodes/vr_openbsd"
	containerlabnodesvr_openwrt "github.com/srl-labs/containerlab/nodes/vr_openwrt"
	containerlabnodesvr_pan "github.com/srl-labs/containerlab/nodes/vr_pan"
	containerlabnodesvr_ros "github.com/srl-labs/containerlab/nodes/vr_ros"
	containerlabnodesvr_sros "github.com/srl-labs/containerlab/nodes/vr_sros"
	containerlabnodesvr_veos "github.com/srl-labs/containerlab/nodes/vr_veos"
	containerlabnodesvr_vjunosevolved "github.com/srl-labs/containerlab/nodes/vr_vjunosevolved"
	containerlabnodesvr_vjunosswitch "github.com/srl-labs/containerlab/nodes/vr_vjunosswitch"
	containerlabnodesvr_vmx "github.com/srl-labs/containerlab/nodes/vr_vmx"
	containerlabnodesvr_vqfx "github.com/srl-labs/containerlab/nodes/vr_vqfx"
	containerlabnodesvr_vsrx "github.com/srl-labs/containerlab/nodes/vr_vsrx"
	containerlabnodesvr_xrv "github.com/srl-labs/containerlab/nodes/vr_xrv"
	containerlabnodesvr_xrv9k "github.com/srl-labs/containerlab/nodes/vr_xrv9k"
	containerlabnodesvyosnetworks_vyos "github.com/srl-labs/containerlab/nodes/vyosnetworks_vyos"
	containerlabnodesxrd "github.com/srl-labs/containerlab/nodes/xrd"
)

// RegisterNodes registers all the nodes/kinds supported by containerlab.
func (c *CLab) RegisterNodes() {
	containerlabnodesbridge.Register(c.Reg)
	containerlabnodesceos.Register(c.Reg)
	containerlabnodescheckpoint_cloudguard.Register(c.Reg)
	containerlabnodescrpd.Register(c.Reg)
	containerlabnodescvx.Register(c.Reg)
	containerlabnodesext_container.Register(c.Reg)
	containerlabnodesfortinet_fortigate.Register(c.Reg)
	containerlabnodeshost.Register(c.Reg)
	containerlabnodesipinfusion_ocnos.Register(c.Reg)
	containerlabnodeskeysight_ixiacone.Register(c.Reg)
	containerlabnodeslinux.Register(c.Reg)
	containerlabnodesovs.Register(c.Reg)
	containerlabnodessonic.Register(c.Reg)
	containerlabnodessrl.Register(c.Reg)
	containerlabnodessros.Register(c.Reg)
	containerlabnodesvr_aoscx.Register(c.Reg)
	containerlabnodesvr_csr.Register(c.Reg)
	containerlabnodesvr_c8000v.Register(c.Reg)
	containerlabnodesvr_freebsd.Register(c.Reg)
	containerlabnodesgeneric_vm.Register(c.Reg)
	containerlabnodesdell_sonic.Register(c.Reg)
	containerlabnodesvr_ftosv.Register(c.Reg)
	containerlabnodesvr_n9kv.Register(c.Reg)
	containerlabnodesvr_pan.Register(c.Reg)
	containerlabnodesvr_openbsd.Register(c.Reg)
	containerlabnodesvr_ftdv.Register(c.Reg)
	containerlabnodesvr_ros.Register(c.Reg)
	containerlabnodesvr_sros.Register(c.Reg)
	containerlabnodesvr_veos.Register(c.Reg)
	containerlabnodesvr_vmx.Register(c.Reg)
	containerlabnodesvr_vsrx.Register(c.Reg)
	containerlabnodesvr_vqfx.Register(c.Reg)
	containerlabnodesvr_vjunosswitch.Register(c.Reg)
	containerlabnodesvr_vjunosevolved.Register(c.Reg)
	containerlabnodesvr_xrv.Register(c.Reg)
	containerlabnodesvr_xrv9k.Register(c.Reg)
	containerlabnodessonic_vm.Register(c.Reg)
	containerlabnodesvr_cat9kv.Register(c.Reg)
	containerlabnodesvr_openwrt.Register(c.Reg)
	containerlabnodesxrd.Register(c.Reg)
	containerlabnodesrare.Register(c.Reg)
	containerlabnodesc8000.Register(c.Reg)
	containerlabnodesk8s_kind.Register(c.Reg)
	containerlabnodesiol.Register(c.Reg)
	containerlabnodeshuawei_vrp.Register(c.Reg)
	containerlabnodes6wind_vsr.Register(c.Reg)
	containerlabnodesfdio_vpp.Register(c.Reg)
	containerlabnodesvyosnetworks_vyos.Register(c.Reg)
	containerlabnodescjunosevolved.Register(c.Reg)
}
