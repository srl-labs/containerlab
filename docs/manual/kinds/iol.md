---
search:
  boost: 4
kind_code_name: cisco_iol
kind_display_name: Cisco IOL
kind_short_display_name: IOL
---
# [[[ kind_display_name ]]]

The Cisco IOL (IOS On Linux) (or [[[ kind_short_display_name ]]] for short) is a version of Cisco IOS/IOS-XE software which doesn't run as a virtual machine. 

Cisco IOL is distributed as a binary and executes directly ontop of Linux, hence the name IOS *On Linux*.

It is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md) and built using [vrnetlab](../vrnetlab.md) project and essentially is the IOL binary packaged into a docker container.

## Getting and building [[[ kind_display_name ]]]

You can get [[[ kind_display_name ]]] from Cisco's CML refplat .iso. It is identified by the `iol` or `ioll2` prefix. 

From the IOL binary you are required to build a container using the [vrnetlab](../vrnetlab.md) project.

IOL is distributed as two versions:

- IOL
    - Meant for usage as an L3 router, lacks L2 switching functionality.
- IOL-L2
    - Meant for usage as a virtual version of an IOS-XE switch. Still has support for some L3 features.

## Resource requirements

[[[ kind_display_name ]]] is very light on resources. There are no strict resource requirements but assume 1vCPU and 1GB of RAM per node at most.

## Managing [[[ kind_display_name ]]] nodes

You can manage the [[[ kind_display_name ]]] with containerlab via the following interfaces:

/// tab | CLI
to connect to the [[[ kind_short_display_name ]]] CLI

```bash
ssh admin@<container-name/id>
```

///
/// tab | bash
to connect to a `bash` shell of a running [[[ kind_short_display_name ]]] container:

```bash
docker exec -it <container-name/id> bash
```

///

/// note
Default credentials: `admin:admin`
///

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in the [[[ kind_display_name ]]] CLI.

The interface naming convention is: `Ethernet0/X` (or `e0/X`), where `X` is the port number.

With that naming convention in mind:

- `e0/1` - first data port available
- `e0/2` - second data port, and so on...

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

- `eth0` - management interface connected to the containerlab management network. Mapped to `Ethernet0/0`.
- `eth1` - First data-plane interface. Mapped to `Ethernet0/1` interface.
- `eth2` - Second data-plane interface. Mapped to `Ethernet0/2` interface and so on.

You must define interfaces in a contigous manner in your toplogy file. For example, if you want to use `Ethernet0/4` you must define `Ethernet0/1`, `Ethernet0/2` and `Ethernet0/3`. See the example below.

```yaml
name: my-iol-lab
topology:
  nodes:
    iol-1:
      kind: cisco_iol
      image: vrnetlab/vr-iol:17.12.01
    iol-2:
      kind: cisco_iol
      image: vrnetlab/vr-iol:17.12.01

  links:
    - endpoints: ["iol-1:Ethernet0/1","iol-2:Ethernet0/1"] 
    - endpoints: ["iol-1:Ethernet0/2","iol-2:Ethernet0/2"]
    - endpoints: ["iol-1:Ethernet0/3", "iol-2:Ethernet0/3"]
    - endpoints: ["iol-1:Ethernet0/4", "iol-2:Ethernet0/4"]
```

/// warning
You may see more interfaces than you have defined in the [[[ kind_short_display_name ]]] CLI, this is because interfaces are provisioned in groups of 4. Links/interfaces that you did not define in your containerlab topology will *not* pass any traffic.
///

When containerlab launches [[[ kind_display_name ]]] node the `Ethernet0/0` interface of the VM gets assigned a management address via DHCP. 

On IOL the `Ethernet0/0` is in it's own management VRF so configuration in the global context will not affect the management interface. This is *not* the case in IOL-L2, applied configuration may interfere with the management interface and take down SSH access to the container.

Data interfaces `Ethernet0/1+` need to be configured with IP addressing manually using CLI or other available management interfaces and will appear `unset` in the CLI:

```
iol#sh ip int br
Interface              IP-Address      OK? Method Status                Protocol
Ethernet0/0            172.20.20.5     YES TFTP   up                    up
Ethernet0/1            unassigned      YES unset  administratively down down
Ethernet0/2            unassigned      YES unset  administratively down down
Ethernet0/3            unassigned      YES unset  administratively down down
```
## Sample topology

Below is a sample topology of two IOL nodes connected via an IOL-L2 switch.

```yaml
name: my-iol-lab
topology:
  nodes:
    iol-router1:
      kind: cisco_iol
      image: vrnetlab/vr-iol:17.12.01
    iol-router2:
      kind: cisco_iol
      image: vrnetlab/vr-iol:17.12.01
    iol-switch:
      kind: cisco_iol
      image: vrnetlab/vr-iol-l2:17.12.01

  links:
    - endpoints: ["iol-router1:Ethernet0/1","iol-switch:Ethernet0/1"] 
    - endpoints: ["iol-router2:Ethernet0/1","iol-switch:Ethernet0/2"]
```