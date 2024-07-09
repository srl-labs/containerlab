# VM-based routers integration

Containerlab focuses on containers, but many routing products ship only in virtual machine packaging. Leaving containerlab users without the ability to create topologies with both containerized and VM-based routing systems would have been a shame.

Keeping this requirement in mind from the very beginning, we added [`bridge`](../lab-examples/ext-bridge.md)/[`ovs-bridge`](kinds/ovs-bridge.md) kind that allows bridging your containerized topology with other resources available via a bridged network. For example, a VM based router:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/vrnetlab.drawio&quot;}"></div>

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

With this approach, you could bridge VM-based routing systems by attaching interfaces to the bridge you define in your topology. However, it doesn't allow users to define the VM-based nodes in the same topology file. With [`vrnetlab`](https://github.com/hellt/vrnetlab) integration, containerlab is now capable of launching topologies with VM-based routers defined in the same topology file.

## Vrnetlab

Vrnetlab packages a regular VM inside a container and makes it runnable as if it was a container image.

To make this work, vrnetlab provides a set of scripts that build the container image out of a user-provided VM disk. This integration enables containerlab to build topologies that consist both of native containerized NOSes and VMs:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/vrnetlab.drawio&quot;}"></div>

!!! warning
    Ensure that the VM that containerlab runs on has [Nested virtualization enabled](https://stafwag.github.io/blog/blog/2018/06/04/nested-virtualization-in-kvm/) to support vrnetlab-based containers.

### Compatibility matrix

To make vrnetlab images to work with container-based networking in containerlab, we needed to [fork](https://github.com/hellt/vrnetlab) vrnetlab project and implement the necessary improvements. VM-based routers that you intend to run with containerlab should be built with [`hellt/vrnetlab`](https://github.com/hellt/vrnetlab) project, and not with the upstream `vrnetlab/vrnetlab`.

Containerlab depends on `hellt/vrnetlab` project, and sometimes features added in containerlab must be implemented in `vrnetlab` (and vice-versa). This leads to a cross-dependency between these projects.

The following table provides a link between the version combinations:

| containerlab[^3] | vrnetlab[^4]                                                       | Notes                                                                                                                                                                  |
| ---------------- | ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `0.10.4`         | [`0.1.0-cl`](https://github.com/hellt/vrnetlab/tree/v0.1.0-cl)     | Initial release. Images: sros, vmx, xrv, xrv9k                                                                                                                         |
| `0.11.0`         | [`0.2.0`](https://github.com/hellt/vrnetlab/tree/v0.2.0)           | added [vr-veos](kinds/vr-veos.md), support for [boot-delay](#boot-delay), SR OS will have a static route to docker network, improved XRv startup chances               |
|                  | [`0.2.1`](https://github.com/hellt/vrnetlab/tree/v0.2.1)           | added timeout for SR OS images to allow eth interfaces to appear in the container namespace. Other images are not touched.                                             |
|                  | [`0.2.2`](https://github.com/hellt/vrnetlab/tree/v0.2.2)           | fixed serial (telnet) access to SR OS nodes                                                                                                                            |
|                  | [`0.2.3`](https://github.com/hellt/vrnetlab/tree/v0.2.3)           | set default cpu/ram for SR OS images                                                                                                                                   |
| `0.13.0`         | [`0.3.0`](https://github.com/hellt/vrnetlab/tree/v0.3.0)           | added support for Cisco CSR1000v via [`cisco_csr`](kinds/vr-csr.md) and MikroTik routeros via [`mikrotik_ros`](kinds/vr-ros.md) kind                                   |
|                  | [`0.3.1`](https://github.com/hellt/vrnetlab/tree/v0.3.1)           | enhanced SR OS boot sequence                                                                                                                                           |
|                  | [`0.4.0`](https://github.com/hellt/vrnetlab/tree/v0.4.0)           | fixed SR OS CPU allocation and added Palo Alto PAN support [`paloaltp_pan`](kinds/vr-pan.md)                                                                           |
| `0.16.0`         | [`0.5.0`](https://github.com/hellt/vrnetlab/tree/v0.5.0)           | added support for Cisco Nexus 9000v via [`cisco_n9kv`](kinds/vr-n9kv.md) kind, added support for non-continuous interfaces provisioning                                |
| `0.19.0`         | [`0.6.0`](https://github.com/hellt/vrnetlab/tree/v0.6.0)           | added experimental support for Juniper vQFX via [`juniper_vqfx`](kinds/vr-vqfx.md) kind, added support Dell FTOS via [`dell_ftosv`](kinds/vr-ftosv.md)                 |
|                  | [`0.6.2`](https://github.com/hellt/vrnetlab/tree/v0.6.2)           | support for IPv6 management for SR OS; support for RouterOS v7+                                                                                                        |
|                  | [`0.7.0`](https://github.com/hellt/vrnetlab/tree/v0.7.0)           | startup-config support for vqfx and vmx                                                                                                                                |
| `0.32.2`         | [`0.8.0`](https://github.com/hellt/vrnetlab/releases/tag/v0.8.0)   | startup-config support for the rest of the kinds, support for multi line card SR OS                                                                                    |
| `0.34.0`         | [`0.8.2`](https://github.com/hellt/vrnetlab/releases/tag/v0.8.2)   | startup-config support for PANOS, ISA support for Nokia VSR-I and MGMT VRF for VMX                                                                                     |
|                  | [`0.9.0`](https://github.com/hellt/vrnetlab/releases/tag/v0.9.0)   | Support for IPInfusion OcNOS with vrnetlab                                                                                                                             |
| `0.41.0`         | [`0.11.0`](https://github.com/hellt/vrnetlab/releases/tag/v0.11.0) | Added support for Juniper vSRX3.0 via [`juniper_vsrx`](kinds/vr-vsrx.md) kind                                                                                          |
| `0.45.0`         | [`0.12.0`](https://github.com/hellt/vrnetlab/releases/tag/v0.12.0) | Added support for Juniper vJunos-switch via [`juniper_vjunosswitch`](kinds/vr-vjunosswitch.md) kind                                                                    |
| `0.49.0`         | [`0.14.0`](https://github.com/hellt/vrnetlab/releases/tag/v0.14.0) | Added support for [Juniper vJunos-Evolved](kinds/vr-vjunosevolved.md), [Cisco FTDv](kinds/vr-ftdv.md), [OpenBSD](kinds/openbsd.md)                                     |
| `0.53.0`         | [`0.15.0`](https://github.com/hellt/vrnetlab/releases/tag/v0.15.0) | Added support for [Fortigate](kinds/fortinet_fortigate.md), [freebsd](kinds/freebsd.md), added lots of FP5 types to Nokia SR OS and support for external cf1/2 disks   |
| `0.54.0`         | [`0.16.0`](https://github.com/hellt/vrnetlab/releases/tag/v0.16.0) | Added support for [Cisco c8000v](kinds/c8000.md)                                                                                                                       |
| `0.55.0`         | [`0.17.0`](https://github.com/hellt/vrnetlab/releases/tag/v0.17.0) | Added support for [Juniper vJunos-router](kinds/vr-vjunosrouter.md), [Generic VM](kinds/generic_vm.md), support for setting qemu parameters via env vars for the nodes |
| `0.56.0`         | [`0.18.1`](https://github.com/hellt/vrnetlab/releases/tag/v0.18.1) | Added support for [Dell SONiC](kinds/dell_sonic.md), [SONiC VM](kinds/sonic-vm.md), [Cisco Catalyst 9000v](kinds/vr-cat9kv.md)                                         |

/// details | how to understand version inter-dependency between containerlab and vrnetlab?
    type: note
When new VM-based platform support is added to vrnetlab, it is usually accompanied by a new containerlab version. In this case the table row will have both containerlab and vrnetlab versions.  
When vrnetlab adds new features that don't require containerlab changes, the table will have only vrnetlab version.  
When containerlab adds new features that don't require vrnetlab changes, the table will not list containerlab version.

It is worth noting, that you can use the latest containerlab version with a given vrnetlab version, even if the table doesn't list the latest containerlab version.
///

### Building vrnetlab images

To build a vrnetlab image compatible with containerlab, users first need to ensure that the versions of both projects follow [compatibility matrix](#compatibility-matrix).

1. Clone [`hellt/vrnetlab`](https://github.com/hellt/vrnetlab) and checkout to a version compatible with containerlab release:

   ```bash
   git clone https://github.com/hellt/vrnetlab && cd vrnetlab
   
   # assuming we are running containerlab 0.11.0,
   # the latest compatible vrnetlab version is 0.2.3
   # at the moment of this writing
   git checkout v0.2.3
   ```

2. Enter the directory for the image of interest

   ```
   cd sros
   ```

3. Follow the build instructions from the README.md file in the image directory

### Supported VM products

The images that work with containerlab will appear in the supported list as we implement the necessary integration.

| Product               | Kind                                                    | Demo lab                                   | Notes                                                                                                                                                                                                        |
| --------------------- | ------------------------------------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Nokia SR OS           | [nokia_sros](kinds/vr-sros.md)                          | [SRL & SR OS](../lab-examples/vr-sros.md)  | When building SR OS vrnetlab image for use with containerlab, **do not** provide the license during the image build process. The license shall be provided in the containerlab topology definition file[^1]. |
| Juniper vMX           | [juniper_vmx](kinds/vr-vmx.md)                          | [SRL & vMX](../lab-examples/vr-vmx.md)     |                                                                                                                                                                                                              |
| Juniper vQFX          | [juniper_vqfx](kinds/vr-vqfx.md)                        |                                            |                                                                                                                                                                                                              |
| Juniper vSRX          | [juniper_vsrx](kinds/vr-vsrx.md)                        |                                            |                                                                                                                                                                                                              |
| Juniper vJunos-Router | [juniper_vjunosrouter](kinds/vr-vjunosrouter.md)        |                                            |                                                                                                                                                                                                              |
| Juniper vJunos-Switch | [juniper_vjunosswitch](kinds/vr-vjunosswitch.md)        |                                            |                                                                                                                                                                                                              |
| Juniper vJunosEvolved | [juniper_vjunosevolved](kinds/vr-vjunosevolved.md)      |                                            |                                                                                                                                                                                                              |
| Cisco XRv             | [cisco_xrv](kinds/vr-xrv.md)                            | [SRL & XRv](../lab-examples/vr-xrv.md)     |                                                                                                                                                                                                              |
| Cisco XRv9k           | [cisco_xrv9k](kinds/vr-xrv9k.md)                        | [SRL & XRv9k](../lab-examples/vr-xrv9k.md) |                                                                                                                                                                                                              |
| Cisco CSR1000v        | [cisco_csr](kinds/vr-csr.md)                            |                                            |                                                                                                                                                                                                              |
| Cisco Nexus 9000v     | [cisco_nexus9kv](kinds/vr-n9kv.md)                      |                                            |                                                                                                                                                                                                              |
| Cisco FTDv            | [cisco_ftdv](kinds/vr-ftdv.md)                          |                                            |                                                                                                                                                                                                              |
| Arista vEOS           | [arista_veos](kinds/vr-veos.md)                         |                                            |                                                                                                                                                                                                              |
| MikroTik RouterOS     | [mikrotik_ros](kinds/vr-ros.md)                         |                                            |                                                                                                                                                                                                              |
| Palo Alto PAN         | [paloalto_pan](kinds/vr-pan.md)                         |                                            |                                                                                                                                                                                                              |
| Dell FTOS10v          | [dell_ftosv](kinds/vr-ftosv.md)                         |                                            |                                                                                                                                                                                                              |
| Aruba AOS-CX          | [aruba_aoscx](kinds/vr-aoscx.md)                        |                                            |                                                                                                                                                                                                              |
| IPInfusion OcNOS      | [ipinfusion_ocnos](kinds/ipinfusion-ocnos.md)           |                                            |                                                                                                                                                                                                              |
| Checkpoint Cloudguard | [checkpoint_cloudguard](kinds/checkpoint_cloudguard.md) |                                            |                                                                                                                                                                                                              |
| Fortinet Fortigate    | [fortinet_fortigate](kinds/fortinet_fortigate.md)       |                                            |                                                                                                                                                                                                              |
| OpenBSD               | [openbsd](kinds/openbsd.md)                             |                                            |                                                                                                                                                                                                              |
| FreeBSD               | [freebsd](kinds/freebsd.md)                             |                                            |                                                                                                                                                                                                              |
| SONiC (VM)            | [sonic-vm](kinds/sonic-vm.md)                           |                                            |                                                                                                                                                                                                              |

### Tuning qemu parameters

When vrnetlab starts a VM inside the container it uses `qemu` command to define the VM parameters such as disk drives, cpu type, memory, etc. Containerlab allows users to tune some of these parameters by setting the environment variables in the topology file. The values from these variables will override defaults set by vrnetlab for this particular VM image.

The following env vars are supported:

- `QEMU_SMP` - sets the number of vCPU cores and their configuration. Use this when the default number of vCPUs is not enough or excessive.
- `QEMU_MEMORY` - sets the amount of memory allocated to the VM in MB. Use this when you want to alter the amount of allocated memory for the VM. Note, that some kinds have a different way to set CPU/MEM parameters, which is explained in the kind's documentation.
- `QEMU_CPU` - sets the default CPU model/type for the node. Use this when the default cpu type is not suitable for your host or you want to experiment with others.
- `QEMU_ADDITIONAL_ARGS` - allows users to pass additional qemu arguments to the VM. These arguments will be appended to the list of the existing arguments. Use this when you need to pass some specific qemu arguments to the VM overriding the defaults set by vrnetlab.

### Connection modes

Containerlab offers several ways of connecting VM-based routers with the rest of the docker workloads. By default, vrnetlab integrated routers will use **tc** backend[^2], which doesn't require any additional packages to be installed on the container host and supports transparent passage of LACP frames.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:6,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/vrnetlab.drawio&quot;}"></div>

??? "Any other datapaths?"
    Although `tc` based datapath should cover all the needed connectivity requirements, if other bridge-like datapaths are needed, Containerlab offers OpenvSwitch and Linux bridge modes.  
    Users can plug in those datapaths by specifying `CONNECTION_MODE` env variable:
    ```yaml
    # the env variable can also be set in the defaults section
    name: myTopo

    topology:
      nodes:
        sr1:
          kind: nokia_sros
          image: vrnetlab/nokia_sros:20.10.R1
          env:
            CONNECTION_MODE: bridge # use `ovs` for openvswitch datapath
    ```

### Networking

Vrnetlab-based container images expose their management interface on the `eth0` interface. Further `eth` interfaces, that is, `eth1` and up, are considered data interfaces and are connected to the VM using `tc` connection mode. Data plane interfaces are connected to the VM preserving both the order of and discontinuity between `eth` data-plane interfaces. Internally, the vrnetlab launch script achieves this by mapping the `eth` interfaces to virtualised NICs at the corresponding PCI bus addresses, filling out gaps with dummy network interfaces.

For example, a vrnetlab node with endpoints `eth2`, `eth3` and `eth5` would have these devices mapped to PCI bus addresses 2, 3 and 5 respectively, while addresses 1 and 4 would be allocated an unconnected (dummy) virtualised NIC.

For convenience and easier adaptation of configurations and lab diagrams to Containerlab topologies, vrnetlab-based nodes also support interface aliasing. Interface aliasing allows for the use of the same interface naming conventions in containerlab topologies as in the NOS, as long as interface aliasing is implemented for the NOS' kind. Note that not all NOS' implementations have support for interface aliases at the moment. For information about the supported interface naming conventions for each NOS, check out their specific [Kinds](../manual/kinds/index.md) page.

### Boot order

A simultaneous boot of many qemu nodes may stress the underlying system, which sometimes renders in a boot loop or system halt. If the container host doesn't have enough capacity to bear the simultaneous boot of many qemu nodes, it is still possible to successfully run them by scheduling their boot time.

Starting with v0.51.0 users may define a "staged" boot process by defining the [`stages`](nodes.md#stages) and `wait-for` dependencies between the VM-based nodes.

Consider the following example where the first SR OS nodes will boot immediately, whereas the second node will wait till the first node is reached the `healthy` stage:

```yaml
name: boot-order
topology:
  nodes:
    sr1:
      kind: nokia_sros
      image: nokia_sros:latest
    sr2:
      kind: nokia_sros
      image: nokia_sros:latest
      stages:
        create:
          wait-for:
            - node: sr1
              stage: healthy
```

### Boot delay

A predecessor of the Boot Order is the boot delay that can be set with `BOOT_DELAY` environment variable that the supported VM-based nodes will respect.

Consider the following example where the first SR OS nodes will boot immediately, whereas the second node will sleep for 30 seconds and then start the boot process:

```yaml
name: boot-delay
topology:
  nodes:
    sr1:
      kind: nokia_sros
      image: nokia_sros:21.2.R1
      license: license-sros21.txt
    sr2:
      kind: nokia_sros
      image: nokia_sros:21.2.R1
      license: license-sros21.txt
      env:
        # boot delay in seconds
        BOOT_DELAY: 30
```

This method is not as flexible as the Boot Order, since you rely on the fixed delay, and it doesn't allow for the dynamic boot order based on the node health.

### Memory optimization

Typically a lab consists of a few types of VMs which are spawned and interconnected with each other. Consider a lab consisting of 5 interconnected routers; one router uses VM image X, and four routers use VM image Y.

Effectively we run just two types of VMs in that lab, and thus we can implement a memory deduplication technique that drastically reduces the memory footprint of a lab. In Linux, this can be achieved with technologies like KSM (via `ksmtuned`). Install KSM package on your distribution and enable it to save memory.

Find some examples below (or contribut a new one)

/// tab | Debian/Ububntu

```bash
sudo apt-get update -y
sudo apt-get install -y ksmtuned

sudo systemctl status ksm.service
sudo systemctl restart ksm.service
sudo echo 1 > /sys/kernel/mm/ksm/run

grep . /sys/kernel/mm/ksm/*
```

If you want KSM always active you could change `#KSM_THRES_COEF=20` in `/etc/ksmtuned.conf` to `KSM_THRES_COEF=99`. That way KSM will kick in as soon as free RAM dops below 99% instead of below the default 20% of free RAM.
///

[^1]: see [this example lab](../lab-examples/vr-sros.md) with a license path provided in the topology definition file
[^2]: pros and cons of different datapaths were examined [here](https://netdevops.me/2021/transparently-redirecting-packetsframes-between-interfaces/)
[^3]: to install a certain version of containerlab, use the [instructions](../install.md) from installation doc.
[^4]: to have a guaranteed compatibility checkout to the mentioned tag and build the images.
