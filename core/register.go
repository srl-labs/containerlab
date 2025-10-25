// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	clabnodes6wind_vsr "github.com/srl-labs/containerlab/nodes/6wind_vsr"
	clabnodesarrcus_arcos "github.com/srl-labs/containerlab/nodes/arrcus_arcos"
	clabnodesbridge "github.com/srl-labs/containerlab/nodes/bridge"
	clabnodesc8000 "github.com/srl-labs/containerlab/nodes/c8000"
	clabnodesceos "github.com/srl-labs/containerlab/nodes/ceos"
	clabnodescheckpoint_cloudguard "github.com/srl-labs/containerlab/nodes/checkpoint_cloudguard"
	clabnodescisco_sdwan "github.com/srl-labs/containerlab/nodes/cisco_sdwan"
	clabnodescjunosevolved "github.com/srl-labs/containerlab/nodes/cjunosevolved"
	clabnodescrpd "github.com/srl-labs/containerlab/nodes/crpd"
	clabnodescvx "github.com/srl-labs/containerlab/nodes/cvx"
	clabnodesdell_sonic "github.com/srl-labs/containerlab/nodes/dell_sonic"
	clabnodesext_container "github.com/srl-labs/containerlab/nodes/ext_container"
	clabnodesfdio_vpp "github.com/srl-labs/containerlab/nodes/fdio_vpp"
	clabnodesfortinet_fortigate "github.com/srl-labs/containerlab/nodes/fortinet_fortigate"
	clabnodesgeneric_vm "github.com/srl-labs/containerlab/nodes/generic_vm"
	clabnodeshost "github.com/srl-labs/containerlab/nodes/host"
	clabnodeshuawei_vrp "github.com/srl-labs/containerlab/nodes/huawei_vrp"
	clabnodesiol "github.com/srl-labs/containerlab/nodes/iol"
	clabnodesipinfusion_ocnos "github.com/srl-labs/containerlab/nodes/ipinfusion_ocnos"
	clabnodesk8s_kind "github.com/srl-labs/containerlab/nodes/k8s_kind"
	clabnodeskeysight_ixiacone "github.com/srl-labs/containerlab/nodes/keysight_ixiacone"
	clabnodeslinux "github.com/srl-labs/containerlab/nodes/linux"
	clabnodesovs "github.com/srl-labs/containerlab/nodes/ovs"
	clabnodesrare "github.com/srl-labs/containerlab/nodes/rare"
	clabnodessonic "github.com/srl-labs/containerlab/nodes/sonic"
	clabnodessonic_vm "github.com/srl-labs/containerlab/nodes/sonic_vm"
	clabnodessrl "github.com/srl-labs/containerlab/nodes/srl"
	clabnodessros "github.com/srl-labs/containerlab/nodes/sros"
	clabnodesvr_aoscx "github.com/srl-labs/containerlab/nodes/vr_aoscx"
	clabnodesvr_c8000v "github.com/srl-labs/containerlab/nodes/vr_c8000v"
	clabnodesvr_cat9kv "github.com/srl-labs/containerlab/nodes/vr_cat9kv"
	clabnodesvr_csr "github.com/srl-labs/containerlab/nodes/vr_csr"
	clabnodesvr_freebsd "github.com/srl-labs/containerlab/nodes/vr_freebsd"
	clabnodescisco_vios "github.com/srl-labs/containerlab/nodes/cisco_vios"
	clabnodescisco_viosl2 "github.com/srl-labs/containerlab/nodes/cisco_viosl2"
	clabnodesvr_ftdv "github.com/srl-labs/containerlab/nodes/vr_ftdv"
	clabnodesvr_ftosv "github.com/srl-labs/containerlab/nodes/vr_ftosv"
	clabnodesvr_n9kv "github.com/srl-labs/containerlab/nodes/vr_n9kv"
	clabnodesvr_openbsd "github.com/srl-labs/containerlab/nodes/vr_openbsd"
	clabnodesvr_openwrt "github.com/srl-labs/containerlab/nodes/vr_openwrt"
	clabnodesvr_pan "github.com/srl-labs/containerlab/nodes/vr_pan"
	clabnodesvr_ros "github.com/srl-labs/containerlab/nodes/vr_ros"
	clabnodesvr_sros "github.com/srl-labs/containerlab/nodes/vr_sros"
	clabnodesvr_veos "github.com/srl-labs/containerlab/nodes/vr_veos"
	clabnodesvr_vjunosevolved "github.com/srl-labs/containerlab/nodes/vr_vjunosevolved"
	clabnodesvr_vjunosswitch "github.com/srl-labs/containerlab/nodes/vr_vjunosswitch"
	clabnodesvr_vmx "github.com/srl-labs/containerlab/nodes/vr_vmx"
	clabnodesvr_vqfx "github.com/srl-labs/containerlab/nodes/vr_vqfx"
	clabnodesvr_vsrx "github.com/srl-labs/containerlab/nodes/vr_vsrx"
	clabnodesvr_xrv "github.com/srl-labs/containerlab/nodes/vr_xrv"
	clabnodesvr_xrv9k "github.com/srl-labs/containerlab/nodes/vr_xrv9k"
	clabnodesvyosnetworks_vyos "github.com/srl-labs/containerlab/nodes/vyosnetworks_vyos"
	clabnodesxrd "github.com/srl-labs/containerlab/nodes/xrd"
)

// RegisterNodes registers all the nodes/kinds supported by containerlab.
func (c *CLab) RegisterNodes() { //nolint:funlen
	clabnodesbridge.Register(c.Reg)
	clabnodesceos.Register(c.Reg)
	clabnodescheckpoint_cloudguard.Register(c.Reg)
	clabnodescisco_sdwan.Register(c.Reg)
	clabnodescrpd.Register(c.Reg)
	clabnodescvx.Register(c.Reg)
	clabnodesext_container.Register(c.Reg)
	clabnodesfortinet_fortigate.Register(c.Reg)
	clabnodeshost.Register(c.Reg)
	clabnodesipinfusion_ocnos.Register(c.Reg)
	clabnodeskeysight_ixiacone.Register(c.Reg)
	clabnodeslinux.Register(c.Reg)
	clabnodesovs.Register(c.Reg)
	clabnodessonic.Register(c.Reg)
	clabnodessrl.Register(c.Reg)
	clabnodessros.Register(c.Reg)
	clabnodesvr_aoscx.Register(c.Reg)
	clabnodesvr_csr.Register(c.Reg)
	clabnodesvr_c8000v.Register(c.Reg)
	clabnodesvr_freebsd.Register(c.Reg)
	clabnodescisco_vios.Register(c.Reg)
	clabnodescisco_viosl2.Register(c.Reg)
	clabnodesgeneric_vm.Register(c.Reg)
	clabnodesdell_sonic.Register(c.Reg)
	clabnodesvr_ftosv.Register(c.Reg)
	clabnodesvr_n9kv.Register(c.Reg)
	clabnodesvr_pan.Register(c.Reg)
	clabnodesvr_openbsd.Register(c.Reg)
	clabnodesvr_ftdv.Register(c.Reg)
	clabnodesvr_ros.Register(c.Reg)
	clabnodesvr_sros.Register(c.Reg)
	clabnodesvr_veos.Register(c.Reg)
	clabnodesvr_vmx.Register(c.Reg)
	clabnodesvr_vsrx.Register(c.Reg)
	clabnodesvr_vqfx.Register(c.Reg)
	clabnodesvr_vjunosswitch.Register(c.Reg)
	clabnodesvr_vjunosevolved.Register(c.Reg)
	clabnodesvr_xrv.Register(c.Reg)
	clabnodesvr_xrv9k.Register(c.Reg)
	clabnodessonic_vm.Register(c.Reg)
	clabnodesvr_cat9kv.Register(c.Reg)
	clabnodesvr_openwrt.Register(c.Reg)
	clabnodesxrd.Register(c.Reg)
	clabnodesrare.Register(c.Reg)
	clabnodesc8000.Register(c.Reg)
	clabnodesk8s_kind.Register(c.Reg)
	clabnodesiol.Register(c.Reg)
	clabnodeshuawei_vrp.Register(c.Reg)
	clabnodes6wind_vsr.Register(c.Reg)
	clabnodesfdio_vpp.Register(c.Reg)
	clabnodesvyosnetworks_vyos.Register(c.Reg)
	clabnodescjunosevolved.Register(c.Reg)
	clabnodesarrcus_arcos.Register(c.Reg)
}
