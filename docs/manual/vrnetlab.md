Containerlab focuses on containers, but there are way more routing products which are only shipped in a virtual machine packaging. Leaving containerlab users without ability to create topologies with both containerized and VM-based routing systems would have been a shame.

Keeping this requirement in mind from the very beginning, we added a kind [`bridge`](../lab-examples/ext-bridge.md), that allows to, ehm, bridge your containerized topology with other resources available via a bridged network. For example a VM based router.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/container-lab/diagrams/vrnetlab.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>

Although this approach has many pros, it doesn't allow users to define the VM based nodes in the same topology file. But not anymore, with [`vrnetlab`](https://github.com/plajjan/vrnetlab) integration containerlab became capable of launching topologies with VM-based routers.

## Vrnetlab
Vrnetlab essentially allows to package a regular VM inside a container and makes it runnable and accessible as if it was a container image.

To make this work, vrnetlab provides a set of scripts that will build the container image taking a user provided qcow file as an input. This enables containerlab to build topologies which consist both of native containerized NOSes and the VMs:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/container-lab/diagrams/vrnetlab.drawio&quot;}"></div>

!!!info
    Although multiple vendors are supported in vrnetlab, to make these images work with container-based networking, we needed to [fork](https://github.com/hellt/vrnetlab) the project and provide the necessary improvements.  
    Thus, the VM based products will appear in the supported list gradually.

Make sure, that the VM that containerlab runs on have [Nested virtualization enabled](https://stafwag.github.io/blog/blog/2018/06/04/nested-virtualization-in-kvm/) to support vrnetlab based containers.

### Supported VM products

#### Nokia SR OS
Nokia's virtualized SR OS, aka VSR/VSim has been added to containerlab supported kinds under the [vr-sros](kinds/vr-sros.md) kind. A [demo lab](../lab-examples/vr-sros.md) explains the way this kind can be used.

To build a container image with SR OS inside users should follow [the provided build instructions](https://github.com/hellt/vrnetlab/tree/master/sros#building-the-docker-image) and using the code of the forked version of a vrnetlab project.

!!!warning
    When building SR OS vrnetlab image for use with containerlab, **do not** provide the license during the image build process. The license shall be provided in the containerlab topology definition file[^1].

#### Juniper vMX
Juniper's virtualized MX router - vMX - has been added to containerlab supported kinds under the [vr-vmx](kinds/vr-vmx.md) kind. A [demo lab](../lab-examples/vr-vmx.md) explains the way this kind can be used.

To build a container image with vMX inside users should follow [the instructions](https://github.com/hellt/vrnetlab/tree/master/vmx#building-the-docker-image) provided and using the code of the forked version of a vrnetlab project.

#### Cisco XRv
Cisco's virtualized XR router (demo) - XRv - has been added to containerlab supported kinds under the [vr-xrv9k](kinds/vr-xrv9k.md) and [vr-xrv](kinds/vr-xrv.md) kinds. The `xr-xrv` kind is added for XRv images which are supreceded by XRv9k images. The reason we keep `vr-xrv` is that it is much more lightweight and can be used for basic control plane interops on a resource constrained hosts.

The [demo lab for xrv9k](../lab-examples/vr-xrv9k.md) and [demo lab for xrv](../lab-examples/vr-xrv.md) explain the way this kinds can be used.

To build a container image with XRv9k/XRv inside users should follow [the instructions](https://github.com/hellt/vrnetlab) provided in the relevant folders and using the code of the forked version of a vrnetlab project.

#### Arista vEOS
Arista's virtualized EOS - vEOS - has been added to containerlab supported kinds under the [vr-veos](kinds/vr-veos.md) 

To build a container image with vEOS inside users should follow [the instructions](https://github.com/hellt/vrnetlab) provided in the relevant folders and using the code of the forked version of a vrnetlab project.


### Connection modes
Containerlab offers several ways VM based routers can be connected with the rest of the docker workloads. By default, vrnetlab integrated routers will use **tc** backend[^2] which doesn't require any additional packages to be installed on the containerhost and supoprts transparent passage of LACP frames.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:6,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/container-lab/diagrams/vrnetlab.drawio&quot;}"></div>

??? "Any other datapaths?"
    Althout `tc` based datapath should cover all the needed connectivity requirements, if other, bridge-like, datapaths are needed, Containerlab offers OpenvSwitch and Linux bridge modes.  
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
Simultaneous boot of many qemu nodes may stress the underlying system, which sometimes render in a boot loop or system halt. If the container host doesn't have enough capacity to bear the simultaneous boot of many qemu nodes it is still possible to successfully run them by scheduling their boot time.

Delaying the boot process of certain nodes by a user defined time will allow nodes to boot successfully while "gradually" load the system. The boot delay can be set with `BOOT_DELAY` environment varialbe that supported `vr-xxxx` kinds will recognize.

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
Typically a lab consists of a few types of VMs which are spawned and inteconnected with each other. Consider a fictious lab that consists of 5 interconnected routers, 1 router uses VM image X and 4 routers are using VM image Y.

Effectively we run just two types of VMs in that lab, and thus we can implement memory deduplication technique that drastically reduces the memory footprint of a lab. In Linux this can be achieved with technologies like UKSM/KSM. Refer to [this article](https://netdevops.me/2021/how-to-patch-ubuntu-20.04-focal-fossa-with-uksm/) that explains the methodology and provides steps to get UKSM working on Ubuntu/Fedora systems.

[^1]: see [this example lab](../lab-examples/vr-sros.md) with a license path provided in the topology definition file
[^2]: pros and cons of different datapaths were examined [here](https://netdevops.me/2021/transparently-redirecting-packets/frames-between-interfaces/)