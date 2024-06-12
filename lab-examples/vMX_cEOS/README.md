Simple Scenario using Juniper vMX and Arista cEOS
With the newer containerlab (version 0.41.2), the kind "juniper_vmx" is not supported. Supported kinds include:

arista_ceos, border0, bridge, c8000, ceos, checkpoint_cloudguard, cisco_c8000, cisco_xrd, crpd, cumulus_cvx, cvx, ext-container, host, ipinfusion_ocnos, juniper_crpd, keysight_ixia-c-one, linux, nokia_srlinux, ovs-bridge, rare, sonic-vs, srl, vr-arista_veos, vr-cisco_csr1000v, vr-cisco_n9kv, vr-cisco_nxos, vr-cisco_xrv, vr-cisco_xrv9k, vr-csr, vr-dell_ftosv, vr-ftosv, vr-juniper_vmx, vr-juniper_vqfx, vr-juniper_vsrx, vr-mikrotik_ros, vr-n9kv, vr-nokia_sros, vr-nxos, vr-paloalto_panos, vr-pan, vr-ros, vr-sros, vr-veos, vr-vmx, vr-vqfx, vr-vsrx, vr-xrv, vr-xrv9k, xrd

This lab scenario utilizes the new kinds to include Juniper vMX and Arista cEOS.

Juniper vMX nodes launched with containerlab come pre-provisioned with SSH, SNMP, NETCONF, and gNMI services enabled.

To add vMX to Containerlab as a Docker image:

Clone the vrnetlab repository:
$ git clone https://github.com/hellt/vrnetlab
Switch to the desired platform, in this case, vMX:
$ cd vrnetlab/vmx
Add the Juniper bundle image to the platform directory.
Run the make command to build the image:
make
This process enables you to include Juniper vMX nodes in your containerlab setup.


