Containerlab focuses on containers, but many routing products ship only in virtual machine packaging. Leaving containerlab users without the ability to create topologies with both containerized and VM-based routing systems would have been a shame.

Keeping this requirement in mind from the very beginning, we added [`bridge`](../lab-examples/ext-bridge.md)/[`ovs-bridge`](kinds/ovs-bridge.md) kind that allows bridging your containerized topology with other resources available via a bridged network. For example, a VM based router:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/vrnetlab.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>

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

| containerlab[^3] | vrnetlab[^4]                                                   | Notes                                                                                                                                                    |
| ---------------- | -------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `0.10.4`         | [`0.1.0-cl`](https://github.com/hellt/vrnetlab/tree/v0.1.0-cl) | Initial release. Images: sros, vmx, xrv, xrv9k                                                                                                           |
| `0.11.0`         | [`0.2.0`](https://github.com/hellt/vrnetlab/tree/v0.2.0)       | added [vr-veos](kinds/vr-veos.md), support for [boot-delay](#boot-delay), SR OS will have a static route to docker network, improved XRv startup chances |
| --               | [`0.2.1`](https://github.com/hellt/vrnetlab/tree/v0.2.1)       | added timeout for SR OS images to allow eth interfaces to appear in the container namespace. Other images are not touched.                               |
| --               | [`0.2.2`](https://github.com/hellt/vrnetlab/tree/v0.2.2)       | fixed serial (telnet) access to SR OS nodes                                                                                                              |
| --               | [`0.2.3`](https://github.com/hellt/vrnetlab/tree/v0.2.3)       | set default cpu/ram for SR OS images                                                                                                                     |
| `0.13.0`         | [`0.3.0`](https://github.com/hellt/vrnetlab/tree/v0.3.0)       | added support for Cisco CSR1000v via [`vr-csr`](kinds/vr-csr.md) and MikroTik routeros via [`vr-ros`](kinds/vr-ros.md) kind                              |
| --               | [`0.3.1`](https://github.com/hellt/vrnetlab/tree/v0.3.1)       | enhanced SR OS boot sequence                                                                                                                             |
| --               | [`0.4.0`](https://github.com/hellt/vrnetlab/tree/v0.4.0)       | fixed SR OS CPU allocation and added Palo Alto PAN support [`vr-pan`](kinds/vr-pan.md)                                                                   |
| `0.16.0`         | [`0.5.0`](https://github.com/hellt/vrnetlab/tree/v0.5.0)       | added support for Cisco Nexus 9000v via [`vr-n9kv`](kinds/vr-n9kv.md) kind, added support for non-continuous interfaces provisioning                     |
| `0.19.0`         | [`0.6.0`](https://github.com/hellt/vrnetlab/tree/v0.6.0)       | added experimental support for Juniper vQFX via [`vr-vqfx`](kinds/vr-vqfx.md) kind, added support Dell FTOS via [`vr-ftosv`](kinds/vr-ftosv.md)          |
|                  | [`0.6.2`](https://github.com/hellt/vrnetlab/tree/v0.6.0)       | support for IPv6 management for SR OS; support for RouterOS v7+                                                                                          |

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

| Product           | Kind                          | Demo lab                                   | Notes                                                                                                                                                                                                        |
| ----------------- | ----------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Nokia SR OS       | [vr-sros](kinds/vr-sros.md)   | [SRL & SR OS](../lab-examples/vr-sros.md)  | When building SR OS vrnetlab image for use with containerlab, **do not** provide the license during the image build process. The license shall be provided in the containerlab topology definition file[^1]. |
| Juniper vMX       | [vr-vmx](kinds/vr-vmx.md)     | [SRL & vMX](../lab-examples/vr-vmx.md)     |                                                                                                                                                                                                              |
| Juniper vQFX      | [vr-vqfx](kinds/vr-vqfx.md)   | Coming soon                                |                                                                                                                                                                                                              |
| Cisco XRv         | [vr-xrv](kinds/vr-xrv.md)     | [SRL & XRv](../lab-examples/vr-xrv.md)     |                                                                                                                                                                                                              |
| Cisco XRv9k       | [vr-xrv9k](kinds/vr-xrv9k.md) | [SRL & XRv9k](../lab-examples/vr-xrv9k.md) |                                                                                                                                                                                                              |
| Cisco CSR1000v    | [vr-csr](kinds/vr-csr.md)     |                                            |                                                                                                                                                                                                              |
| Arista vEOS       | [vr-veos](kinds/vr-veos.md)   |                                            |                                                                                                                                                                                                              |
| MikroTik RouterOS | [vr-ros](kinds/vr-ros.md)     |                                            |                                                                                                                                                                                                              |
| Palo Alto PAN     | [vr-pan](kinds/vr-pan.md)     |                                            |                                                                                                                                                                                                              |
| Cisco Nexus 9000v | [vr-n9kv](kinds/vr-n9kv.md)   |                                            |                                                                                                                                                                                                              |
| Dell FTOS10v      | [vr-ftosv](kinds/vr-ftosv.md) |                                            |                                                                                                                                                                                                              |




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
          kind: vr-sros
          image: vrnetlab/vr-sros:20.10.R1
          env:
            CONNECTION_MODE: bridge # use `ovs` for openvswitch datapath
    ```

### Boot delay
A simultaneous boot of many qemu nodes may stress the underlying system, which sometimes renders in a boot loop or system halt. If the container host doesn't have enough capacity to bear the simultaneous boot of many qemu nodes, it is still possible to successfully run them by scheduling their boot time.

Delaying the boot process of specific nodes by a user-defined time will allow nodes to boot successfully while "gradually" loading the system. The boot delay can be set with `BOOT_DELAY` environment variable that supported `vr-xxxx` kinds will recognize.

Consider the following example where the first SR OS nodes will boot immediately, whereas the second node will sleep for 30 seconds and then start the boot process:

```yaml
name: bootdelay
topology:
  nodes:
    sr1:
      kind: vr-sros
      image: vr-sros:21.2.R1
      license: license-sros21.txt
    sr2:
      kind: vr-sros
      image: vr-sros:21.2.R1
      license: license-sros21.txt
      env:
        # boot delay in seconds
        BOOT_DELAY: 30
```

### Memory optimization
Typically a lab consists of a few types of VMs which are spawned and interconnected with each other. Consider a lab consisting of 5 interconnected routers; one router uses VM image X, and four routers use VM image Y.

Effectively we run just two types of VMs in that lab, and thus we can implement a memory deduplication technique that drastically reduces the memory footprint of a lab. In Linux, this can be achieved with technologies like UKSM/KSM. Refer to [this article](https://netdevops.me/2021/how-to-patch-ubuntu-20.04-focal-fossa-with-uksm/) that explains the methodology and provides steps to get UKSM working on Ubuntu/Fedora systems.

[^1]: see [this example lab](../lab-examples/vr-sros.md) with a license path provided in the topology definition file
[^2]: pros and cons of different datapaths were examined [here](https://netdevops.me/2021/transparently-redirecting-packets/frames-between-interfaces/)
[^3]: to install a certain version of containerlab, use the [instructions](../install.md) from installation doc.
[^4]: to have a guaranteed compatibility checkout to the mentioned tag and build the images.
