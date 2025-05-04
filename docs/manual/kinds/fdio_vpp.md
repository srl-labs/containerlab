---
search:
  boost: 4
---
# FD.io VPP

## Quickstart

A simple lab showing two VPP instances and two Alpine Linux clients can be found on
[git.ipng.ch/ipng/vpp-containerlab](https://git.ipng.ch/ipng/vpp-containerlab). Simply check out the
repo and start the lab, like so:

```
$ git clone https://git.ipng.ch/ipng/vpp-containerlab.git
$ cd vpp-containerlab
$ containerlab deploy --topo vpp.clab.yml
```

Take a look at the repo's [README](https://git.ipng.ch/ipng/vpp-containerlab) for a quick tour.

## VPP Containerlab

There are three moving parts to the VPP container: the `docker image`, the `dataplane` with its
configuration `vppcfg`, and the `controlplane` with its configuration in `bird2`.

### 1. Docker Image

A VPP Containerlab image, including source and build instructions, is provided by IPng Networks on
their [Git Repo](https://git.ipng.ch/ipng/vpp-containerlab). Images are published each time a
VPP release is completed, using Debian Bookworm base image, and VPP Debian packages from
[FD.io](https://fd.io)'s official release repository at
[Packagecloud.io](https://packagecloud.io/app/fdio/release/search).

The resulting Docker image is available at `git.ipng.ch/ipng/vpp-containerlab:latest`.

### 2. Dataplane

#### VPP Startup

The container runs the VPP binary with a given configuration file, in which VPP is told which
plugins to run, how much memory to use, how many threads to start, and so on. Somewhat
unfortunately, in VPP this configuration file is called `startup.conf` but specifically _does not_
contain any runtime configuration. The VPP dataplane does not come with any configuration
persistence mechanism, and only offers a programmable API to do things like set up interfaces,
sub-interfaces, IP addresses, routes, MPLS entries, ACLs and so on.  You will specifically *not*
find an equivalent of `write mem` on VPP. Runtime configuration is left as an exercise for
integrators.

#### VPP Configuration

The Containerlab image ships with [vppcfg](https://git.ipng.ch/ipng/vppcfg), a utility
that takes a YAML configuration file, checks it for syntax and semantic correctness, and then
reconciles a running VPP dataplane with its configuration. It is meant to be re-entrant and
stateless. This tool connects to the VPP API and creates/removes all of the configuration in a
minimally intrusive way.

Then the container starts, it reads `/etc/vpp/vppcfg.yaml` and if it is syntactically correct,
programs the VPP API to reflect the given configuration. The file can be edited, after which
`vppcfg` can prepare the configuration changes needed to bring the VPP dataplane to the new
configuration, like so:

```
root@clab-vpp:~# vppcfg plan -c /etc/vpp/vppcfg.yaml -o /etc/vpp/vppcfg.vpp
root@clab-vpp:~# vppctl exec /etc/vpp/vppcfg.vpp
```

For more details on `vppcfg`, see its [Config Guide](https://git.ipng.ch/ipng/vppcfg/blob/main/docs/config-guide.md).

### 3. Controlplane

#### VPP Linux Control Plane

The container creates and runs VPP in its own dedicated network namespace, called `dataplane`. 
Using a VPP plugin called the Linux Control Plane, it can share interfaces between the Linux
kernel and VPP itself. Any unicast/multicast traffic that is destined for an address on such a
shared interface, is copied to ther kernel, and any traffic coming from the kernel is picked up and
routed. In this way, "North/South" traffic such as OSPF, BGP or VRRP, can be handled by controlplane
software, while traffic that is transitting through the router stays in the (much faster) dataplane.

#### Working with `default` and `dataplane` namespaces

Namespaces in Linux are very similar to `network-instances` in SRLinux, or `VRFs` in IOS/XR or JunOS.
They keep an isolated copy of a network stack, each with their own interfaces, routing tables, and
so on.

The controlplane software used in this container runs in the `dataplane` network namespace, where it
interacts with VPP using the Netlink API. In this `dataplane` namespace, changes to interfaces (like
link admin-state, MTU, IP addresses and routes) are picked up automatically by VPP's Linux Control
Plane plugin, and the kernel routing table is kept in sync with the dataplane.

You can enter the VPP network namespace using `nsenter`, like so:

```
root@clab-vpp:~# ip -br a
lo               UNKNOWN 127.0.0.1/8 ::1/128 
eth0@if531227    UP      172.20.20.3/24 3fff:172:20:20::3/64 fe80::42:acff:fe14:1403/64 
eth1@if531235    UP             
eth2@if531236    UP             
root@clab-vpp:~# nsenter --net=/var/run/netns/dataplane
root@clab-vpp:~# ip -br a
lo               UNKNOWN 127.0.0.1/8 ::1/128
loop0            UP      10.82.98.0/32 2001:db8:8298::/128 fe80::dcad:ff:fe00:0/64 
eth1             UP      10.82.98.65/28 2001:db8:8298:101::1/64 fe80::a8c1:abff:fe77:acb9/64 
eth2             UP      10.82.98.16/31 2001:db8:8298:1::1/64 fe80::a8c1:abff:fef0:7125/64                                   
```

Note that in the default namespace, `eth1@ifX` and `eth2@ifY` are links to other `nodes`, while
`eth0@ifZ` is the managment network interface. But in the `dataplane` namespace, VPP's Linux Control
Plane plugin has created an `loop0`, `eth1`, and `eth2` interfaces and configured them with some
IPv4 and IPv6 addresses.

#### Bird2 Controlplane

The controlplane agent used in this image is Bird2, which can be configured by the user by means of
a bind-mount in `/etc/bird/bird-local.conf`. Edits to this file can be tested and applied like so:

```
root@clab-vpp:~# bird -c /etc/bird/bird.conf -p && birdc configure
root@clab-vpp:~# birdc show protocol
BIRD 2.0.12 ready.
Name       Proto      Table      State  Since         Info
device1    Device     ---        up     2025-05-04 10:23:17  
direct1    Direct     ---        up     2025-05-04 10:23:17  
kernel4    Kernel     master4    up     2025-05-04 10:23:17  
kernel6    Kernel     master6    up     2025-05-04 10:23:17  
bfd1       BFD        ---        up     2025-05-04 10:23:17  
ospf4      OSPF       master4    up     2025-05-04 10:23:17  Running
ospf6      OSPF       master6    up     2025-05-04 10:23:17  Running
```

The first call checks to see if `bird.conf`, which includes the user specified `bird-local.conf`, is
syntactically correct and if so, the configuration is reloaded. If not, an error will be printed.
The second call shows the routing protocols running in Bird2. Note that these routing protocols are
running in the `dataplane` network namespace, so you need to use `nsenter` to join that namespace in
order to do things like inspect the routing table.

For more details on `bird2`, see its [User Guide](https://bird.network.cz/?get_doc&f=bird.html&v=20).

## Advanced Configuration

There are a few places where you can grab more control of the container.

### Alternate `bootstrap.vpp`

When VPP starts, its configuration instructs it to execute all commands found in
`/etc/vpp/bootstrap.vpp` one by one on the `vppctl` commandline. In the default configuration, the
`bootstrap.vpp` executes two files in order:

* `clab.vpp` -- generated by `/sbin/init-container.sh`. Its purpose is to bind the `veth`
  interfaces that Containerlab has added to the container into the VPP dataplane.
* `vppcfg.vpp` -- generated by `/sbin/init-container.sh`. Its purpose is to read the user
  specified `vppcfg.yaml` file and convert it into VPP CLI commands. If no YAML file is
  specified, or if it is not syntactically valid, an empty file is generated instead.

If you bind-mount `/etc/vpp/bootstrap.vpp`, you can take full control over the dataplane startup.

### Alternate `startup.conf`

The VPP binary config file (not the runtime configuration!) can be overridden by bind-mounting
`/etc/vpp/startup.conf`. You can use this to enable/disable certain plugins or change runtime
configuration like memory, CPU threads, and so on.

### Alternate `init-container.sh`

The Docker container starts up with an entrypoint of `/sbin/init-container.sh`, which organizes the
runtime, starts sshd and bird, and prepares the VPP configs. To take full control of the container,
you can bind-mount it into a different executable.

### Alternate Docker container

Take a look at IPng's [README](https://git.ipng.ch/ipng/vpp-containerlab) for pointers on how to
create and test your own Docker image for use with Container lab.
