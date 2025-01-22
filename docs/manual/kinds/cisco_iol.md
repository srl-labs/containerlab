---
search:
  boost: 4
kind_code_name: cisco_iol
kind_display_name: Cisco IOL
kind_short_display_name: IOL
---
# -{{ kind_display_name }}-

-{{ kind_display_name }}- (IOS On Linux or -{{ kind_short_display_name }}- for short) is a version of Cisco IOS/IOS-XE software which is packaged as binary, in other words it does not require a virtual machine, hence the name IOS *On Linux*.

It is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md) and built using [vrnetlab](../vrnetlab.md) project and essentially is the IOL binary packaged into a docker container.

## Getting and building -{{ kind_display_name }}-

You can get -{{ kind_display_name }}- from Cisco's CML refplat .iso. It is identified by the `iol` or `ioll2` prefix.

From the IOL binary you are required to build a container using the [vrnetlab](../vrnetlab.md) project.

IOL is distributed as two versions:

- **IOL** - For usage as an L3 router, lacks L2 switching functionality.
- **IOL-L2** - For usage as a virtual version of an IOS-XE switch. Still has support for some L3 features. See [usage information](#usage-and-sample-topology).

## Resource requirements

-{{ kind_display_name }}- is very light on resources compared to VM-based Cisco products. Each IOL node requires at minimum:

- 1vCPU per node, you are able oversubscribe and run many IOL nodes per vCPU.
- 768Mb of RAM.
- 1Mb of disk space for the NVRAM (where configuration is saved).

Using [KSM](../vrnetlab.md#memory-optimization) you can achieve a higher density of IOL nodes per GB of RAM.

## Managing -{{ kind_display_name }}- nodes

You can manage the -{{ kind_display_name }}- with containerlab via the following interfaces:

/// tab | CLI
to connect to the -{{ kind_short_display_name }}- CLI

```bash
ssh admin@<container-name/id>
```

///
/// tab | bash
to connect to a `bash` shell of a running -{{ kind_short_display_name }}- container:

```bash
docker exec -it <container-name/id> bash
```

///

/// note
Default credentials: `admin:admin`
///

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in the -{{ kind_display_name }}- CLI.

The interface naming convention is: `Ethernet0/X` (or `e0/X`), where `X` is the port number.

With that naming convention in mind:

- `e0/1` - First data-plane interface available
- `e0/2` - Second data-plane interface, and so on...

Keep in mind IOL defines interfaces in groups of 4. Every four interfaces (zero-indexed), the slot index increments by one.

- `e0/3` - Third data-plane interface
- `e1/0` - Fourth data-plane interface
- `e1/1` - Fifth data-plane interface

The example ports above would be mapped to the following Linux interfaces inside the container running -{{ kind_display_name }}-:

- `eth0` - management interface connected to the containerlab management network. Mapped to `Ethernet0/0`.
- `eth1` - First data-plane interface. Mapped to `Ethernet0/1` interface.
- `eth2` - Second data-plane interface. Mapped to `Ethernet0/2` interface
- `eth3` - Third data-plane interface. Mapped to `Ethernet0/3` interface
- `eth4` - Fourth data-plane interface. Mapped to `Ethernet1/0` interface
- `eth5` - Fifth data-plane interface. Mapped to `Ethernet1/1` interface and so on...

When containerlab launches -{{ kind_display_name }}-, the `Ethernet0/0` interface of the container gets assigned management IPv4 and IPv6 addresses from docker. The `Ethernet0/0` interface is in it's own management VRF so that configuration in the global context will not affect the management interface.

Interfaces can be defined in a non-contigous manner in your toplogy file. See the example below.

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
    - endpoints: ["iol-1:Ethernet1/3", "iol-2:Ethernet1/0"]
```

/// warning
When defining interfaces non-contigiously you may see more interfaces than you have defined in the -{{ kind_short_display_name }}- CLI, this is because interfaces are provisioned in groups.

At minimum you will see all numerically-lower indexed interfaces in the CLI compared to the interface you have defined, you may also see interfaces with a higher numerical index.

**Links/interfaces that you did not define in your containerlab topology will *not* pass any traffic.**
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

## Startup configuration

When -{{ kind_display_name }}- is booted, it will start with a basic configuration which configures the following:

- IP addressing for the Ethernet0/0 (management) interface.
- Management VRF for the Ethernet0/0 interface.
- Default route(s) in the management VRF context for the [management network](../network.md#management-network).
- SSH server.
- Sets all user defined interfaces into 'up' state.

On subsequent boots (deployments which are not the first boot of -{{ kind_short_display_name }}-), -{{ kind_short_display_name }}- will take a few extra seconds to come up, this is because Containerlab must update the management interface IP addressing and default routes for the management network.

### User-defined config

-{{ kind_display_name }}- supports user defined startup configurations in two forms:

- Full startup configuration.
- Partial startup configuration.

Both types of startup configurations are only be applied on the **first boot** of -{{ kind_short_display_name }}-. When you save configuration in IOL to the NVRAM (using `write memory` or `copy run start` commands), the NVRAM configuration will override the startup configuration.

#### Full startup configuration

The full startup configuration is used to fully replace/override the default startup configuration that is applied. This means you must define IP addressing and the SSH server in your configuration to access -{{ kind_short_display_name }}-.

You can use the template variables that are defined in the [default startup confguration](https://github.com/srl-labs/containerlab/blob/main/nodes/iol/iol.cfg.tmpl). On lab deployment the template variables will be replaced/substituted.

```yaml
name: iol_full_startup_cfg
topology:
  nodes:
    iol:
      kind: cisco_iol
      startup-config: configuration.txt
```

#### Partial  startup configuration

The partial startup configuration is appended to the default startup configuration. This is useful to preconfigure certain things like loopback interfaces or IGP, while also taking advantage of the startup configuration that containerlab applies by default for management interface IP addressing and SSH access.

The partial startup configuration must contain `.partial` in the filename. For example: `config.partial.txt` or `config.partial`

```yaml
name: iol_partial_startup_cfg
topology:
  nodes:
    iol:
      kind: cisco_iol
      startup-config: configuration.txt.partial
```


## Usage and sample topology

IOL-L2 requires a different startup configuration compared to the regular IOL. You can tell containerlab you are using the L2 image by supplying the `type` field in your topology.

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
      type: L2
  links:
    - endpoints: ["router1:Ethernet0/1","switch:Ethernet0/1"]
    - endpoints: ["router2:Ethernet0/1","switch:e0/2"]
```
