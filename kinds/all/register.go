// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package all

import (
	"github.com/srl-labs/containerlab/kinds"
	"github.com/srl-labs/containerlab/kinds/types/border0"
	"github.com/srl-labs/containerlab/kinds/types/bridge"
	"github.com/srl-labs/containerlab/kinds/types/c8000"
	"github.com/srl-labs/containerlab/kinds/types/ceos"
	"github.com/srl-labs/containerlab/kinds/types/checkpoint_cloudguard"
	"github.com/srl-labs/containerlab/kinds/types/crpd"
	"github.com/srl-labs/containerlab/kinds/types/cvx"
	"github.com/srl-labs/containerlab/kinds/types/dell_sonic"
	"github.com/srl-labs/containerlab/kinds/types/ext_container"
	"github.com/srl-labs/containerlab/kinds/types/fortinet_fortigate"
	"github.com/srl-labs/containerlab/kinds/types/generic_vm"
	"github.com/srl-labs/containerlab/kinds/types/host"
	"github.com/srl-labs/containerlab/kinds/types/huawei_vrp"
	"github.com/srl-labs/containerlab/kinds/types/iol"
	"github.com/srl-labs/containerlab/kinds/types/ipinfusion_ocnos"
	"github.com/srl-labs/containerlab/kinds/types/k8s_kind"
	"github.com/srl-labs/containerlab/kinds/types/keysight_ixiacone"
	"github.com/srl-labs/containerlab/kinds/types/linux"
	"github.com/srl-labs/containerlab/kinds/types/ovs"
	"github.com/srl-labs/containerlab/kinds/types/rare"
	"github.com/srl-labs/containerlab/kinds/types/sonic"
	vr_sonic "github.com/srl-labs/containerlab/kinds/types/sonic_vm"
	"github.com/srl-labs/containerlab/kinds/types/srl"
	"github.com/srl-labs/containerlab/kinds/types/vr_aoscx"
	"github.com/srl-labs/containerlab/kinds/types/vr_c8000v"
	"github.com/srl-labs/containerlab/kinds/types/vr_cat9kv"
	"github.com/srl-labs/containerlab/kinds/types/vr_csr"
	"github.com/srl-labs/containerlab/kinds/types/vr_freebsd"
	"github.com/srl-labs/containerlab/kinds/types/vr_ftdv"
	"github.com/srl-labs/containerlab/kinds/types/vr_ftosv"
	"github.com/srl-labs/containerlab/kinds/types/vr_n9kv"
	"github.com/srl-labs/containerlab/kinds/types/vr_openbsd"
	"github.com/srl-labs/containerlab/kinds/types/vr_pan"
	"github.com/srl-labs/containerlab/kinds/types/vr_ros"
	"github.com/srl-labs/containerlab/kinds/types/vr_sros"
	"github.com/srl-labs/containerlab/kinds/types/vr_veos"
	"github.com/srl-labs/containerlab/kinds/types/vr_vjunosevolved"
	"github.com/srl-labs/containerlab/kinds/types/vr_vjunosswitch"
	"github.com/srl-labs/containerlab/kinds/types/vr_vmx"
	"github.com/srl-labs/containerlab/kinds/types/vr_vqfx"
	"github.com/srl-labs/containerlab/kinds/types/vr_vsrx"
	"github.com/srl-labs/containerlab/kinds/types/vr_xrv"
	"github.com/srl-labs/containerlab/kinds/types/vr_xrv9k"
	"github.com/srl-labs/containerlab/kinds/types/xrd"
)

var Registry = kinds.NewRegistry()
var localRegistryBuilder = kinds.RegistryBuilder{
	bridge.AddToRegistry,
	ceos.AddToRegistry,
	checkpoint_cloudguard.AddToRegistry,
	crpd.AddToRegistry,
	cvx.AddToRegistry,
	ext_container.AddToRegistry,
	fortinet_fortigate.AddToRegistry,
	host.AddToRegistry,
	ipinfusion_ocnos.AddToRegistry,
	keysight_ixiacone.AddToRegistry,
	linux.AddToRegistry,
	ovs.AddToRegistry,
	sonic.AddToRegistry,
	srl.AddToRegistry,
	vr_aoscx.AddToRegistry,
	vr_csr.AddToRegistry,
	vr_c8000v.AddToRegistry,
	vr_freebsd.AddToRegistry,
	generic_vm.AddToRegistry,
	dell_sonic.AddToRegistry,
	vr_ftosv.AddToRegistry,
	vr_n9kv.AddToRegistry,
	vr_pan.AddToRegistry,
	vr_openbsd.AddToRegistry,
	vr_ftdv.AddToRegistry,
	vr_ros.AddToRegistry,
	vr_sros.AddToRegistry,
	vr_veos.AddToRegistry,
	vr_vmx.AddToRegistry,
	vr_vsrx.AddToRegistry,
	vr_vqfx.AddToRegistry,
	vr_vjunosswitch.AddToRegistry,
	vr_vjunosevolved.AddToRegistry,
	vr_xrv.AddToRegistry,
	vr_xrv9k.AddToRegistry,
	vr_sonic.AddToRegistry,
	vr_cat9kv.AddToRegistry,
	xrd.AddToRegistry,
	rare.AddToRegistry,
	c8000.AddToRegistry,
	border0.AddToRegistry,
	k8s_kind.AddToRegistry,
	cisco_iol.AddToRegistry,
	huawei_vrp.AddToRegistry,
}

var AddToRegistry = localRegistryBuilder.AddToRegistry

func init() {
	if err := AddToRegistry(Registry); err != nil {
		panic(err)
	}
}
