---
search:
  boost: 4
kind_code_name: cisco_iol
kind_display_name: Cisco IOL
kind_short_display_name: IOL
---
# [[[ kind_display_name ]]]

[[[ kind_display_name ]]] (IOS On Linux or [[[ kind_short_display_name ]]] for short) is a version of Cisco IOS/IOS-XE software which is packaged as binary, in other words it does not require a virtual machine, hence the name IOS *On Linux*.

It is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md) and built using [vrnetlab](../vrnetlab.md) project and essentially is the IOL binary packaged into a docker container.

## Getting and building [[[ kind_display_name ]]]

You can get [[[ kind_display_name ]]] from Cisco's CML refplat .iso. It is identified by the `iol` or `ioll2` prefix.

From the IOL binary you are required to build a container using the [vrnetlab](../vrnetlab.md) project.

IOL is distributed as two versions:

- **IOL** - For usage as an L3 router, lacks L2 switching functionality.
- **IOL-L2** - For usage as a virtual version of an IOS-XE switch. Still has support for some L3 features. See [usage information](#usage-and-sample-topology).

## Resource requirements

[[[ kind_display_name ]]] is very light on resources compared to VM-based Cisco products. Each IOL node requires at minimum 1Mb of disk space for the NVRAM (where configuration is saved) and 768M of RAM. Assume 1vCPU per node, but you can oversubscribe and run multiple IOL nodes per vCPU.

Using [KSM](../vrnetlab.md#memory-optimization) you can achieve a higher density of IOL nodes per GB of RAM.

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

The example ports above would be mapped to the following Linux interfaces inside the container running [[[ kind_display_name ]]]:

- `eth0` - management interface connected to the containerlab management network. Mapped to `Ethernet0/0`.
- `eth1` - First data-plane interface. Mapped to `Ethernet0/1` interface.
- `eth2` - Second data-plane interface. Mapped to `Ethernet0/2` interface and so on.

When containerlab launches [[[ kind_display_name ]]], the `Ethernet0/0` or `Vlan1` interface of the container gets assigned management IPv4 and IPv6 addresses from docker. On IOL the `Ethernet0/0` is in it's own management VRF so configuration in the global context will not affect the management interface. On IOL-L2 the management interface is the `Vlan1` interface, it is also in it's own management VRF.

Interfaces must be defined in a contigous manner in your toplogy file. For example, if you want to use `Ethernet0/4` you must define `Ethernet0/1`, `Ethernet0/2` and `Ethernet0/3`. See the example below.

```yaml
name: my-iol-lab
topology:
  nodes:
    iol-1:
      kind: cisco_iol
      image: vrnetlab/cisco_iol:17.12.01
    iol-2:
      kind: cisco_iol
      image: vrnetlab/cisco_iol:17.12.01

  links:
    - endpoints: ["iol-1:Ethernet0/1","iol-2:Ethernet0/1"] 
    - endpoints: ["iol-1:Ethernet0/2","iol-2:Ethernet0/2"]
    - endpoints: ["iol-1:Ethernet0/3", "iol-2:Ethernet0/3"]
    - endpoints: ["iol-1:Ethernet0/4", "iol-2:Ethernet0/4"]
```

/// warning
You may see more interfaces than you have defined in the [[[ kind_short_display_name ]]] CLI, this is because interfaces are provisioned in groups. Links/interfaces that you did not define in your containerlab topology will *not* pass any traffic.
///

Data interfaces `Ethernet0/1+` need to be configured with IP addressing manually using CLI or other available management interfaces and will appear `unset` in the CLI:

```
iol#sh ip int br
Interface              IP-Address      OK? Method Status                Protocol
Ethernet0/0            172.20.20.5     YES TFTP   up                    up
Ethernet0/1            unassigned      YES unset  administratively down down
Ethernet0/2            unassigned      YES unset  administratively down down
Ethernet0/3            unassigned      YES unset  administratively down down
```

## Usage and sample topology

IOL-L2 has a different startup configuration compared to the regular IOL. You can tell containerlab you are using the L2 image by supplying the `type` field in your topology. 

See the sample topology below

```yaml
name: iol
topology:
  nodes:
    router1:
      kind: cisco_iol
      image: vrnetlab/cisco_iol:17.12.01
    router2:
      kind: cisco_iol
      image: vrnetlab/cisco_iol:17.12.01
    switch:
      kind: cisco_iol
      image: vrnetlab/cisco_iol:L2-17.12.01
      type: l2
  links:
    - endpoints: ["router1:Ethernet0/1","switch:Ethernet0/1"]
    - endpoints: ["router2:Ethernet0/1","switch:e0/2"]
```
