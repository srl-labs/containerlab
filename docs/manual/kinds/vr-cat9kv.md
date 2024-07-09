---
search:
  boost: 4
kind_code_name: cisco_cat9kv
kind_display_name: Cisco Catalyst 9000v
kind_short_display_name: Cat9kv
---
# Cisco Catalyst 9000v

The [[[ kind_display_name ]]] (or [[[ kind_short_display_name ]]] for short) is a virtualised form of the Cisco Catalyst 9000 series switches. It is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

The [[[ kind_display_name ]]] performs simulation of the dataplane ASICs that are present in the physical hardware. The two simulated ASICs are:

- Cisco UADP (Unified Access Data-Plane). This is the default ASIC that's simulated.
- Silicon One Q200 (referred to as Q200).

/// note
The Q200 simulation has a limited featureset compared to the UADP simulation.
///

## Resource requirements

|           | UADP  | Q200  |
| --------- | ----- | ----- |
| vCPU      | 4     | 4     |
| RAM (MB)  | 18432 | 12288 |
| Disk (GB) | 4     | 4     |

## Managing [[[ kind_display_name ]]] nodes

You can manage the [[[ kind_display_name ]]] with containerlab via the following interfaces:

/// tab | bash
to connect to a `bash` shell of a running Cisco CSR1000v container:

```bash
docker exec -it <container-name/id> bash
```

///
/// tab | CLI
to connect to the Catalyst 9000v CLI

```bash
ssh admin@<container-name/id>
```

///
/// tab | NETCONF
NETCONF server is running over port 830

```bash
ssh admin@<container-name> -p 830 -s netconf
```

///
/// note
Default credentials: `admin:admin`
///

## Interface naming convention

The Cisco Catalyst 9000v container uses the following naming convention for its management and data interfaces:

- `eth0` - management interface connected to the containerlab management network.
- `eth1` - GigabitEthernet1/0/1 interface.
- `eth2` - GigabitEthernet1/0/2 interface and so on.

Regardless of how many links are defined in your containerlab topology, the Catalyst 9000v will always display 8 data-plane interfaces. Links/interfaces that you did not define in your containerlab topology will *not* pass any traffic.

/// note
Data interfaces may take 5+ minutes to come up after the node boots.
///

## Features and options

### ASIC data-plane simulation configuration

The default ASIC simulation of the node will be UADP. To enable the Q200 simulation or to enable specific features for the UADP simulation, you must provide a `vswitch.xml` file (with the relevant configuration) when building the image using [vrnetlab](../vrnetlab.md).

Once the node has been built you are unable to chang the simulation type. Please refer to the README file in vrnetlab/cat9kv for more information.

/// note
You can obtain a `vswitch.xml` file from the relevant Cisco CML node definitions.
///

### Environment variables

There are `VCPU` and `RAM` environment variables defined. It is not recommended reduce the resources below the required amount. The node will be unable to boot in this case.

The example below assigns 6vCPUs and 20 gigabytes of RAM to the node.

```yaml
name: my-cat9kv-lab
topology:
  nodes:
    node1:
      kind: cisco_cat9kv
      image: vrnetlab/vr-cat9kv:17.12.01p-UADP
    env:
     VCPU: 6
     RAM: 20480
```
