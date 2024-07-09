---
search:
  boost: 4
kind_code_name: cisco_cat9kv
kind_display_name: Cisco Catalyst 9000v
kind_short_display_name: Cat9kv
---
# Cisco Catalyst 9000v

The [[[ kind_display_name ]]] (or [[[ kind_short_display_name ]]] for short) is a virtualised form of the Cisco Catalyst 9000 series switches. It is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md) and built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

The [[[ kind_display_name ]]] performs simulation of the dataplane ASICs that are present in the physical hardware. The two simulated ASICs are:

- Cisco UADP (Unified Access Data-Plane). This is the default ASIC that's simulated.
- Silicon One Q200 (referred to as Q200).

/// note
The Q200 simulation has a limited featureset compared to the UADP simulation.
///

## Resource requirements

The [[[ kind_display_name ]]] is a resource-hungry VM. When launched with the default settings, it requires the following resources:

|           | UADP  | Q200  |
| --------- | ----- | ----- |
| vCPU      | 4     | 4     |
| RAM (MB)  | 18432 | 12288 |
| Disk (GB) | 4     | 4     |

Users can adjust the CPU and memory resources by setting adding appropriate environment variables as explained in [Tuning Qemu Parameters section](../../manual/vrnetlab.md#tuning-qemu-parameters).

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

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in the [[[ kind_display_name ]]] CLI.

The interface naming convention is: `GigabitEthernet1/0/X` (or `Gi1/0/X`), where `X` is the port number.

With that naming convention in mind:

- `Gi1/0/1` - first data port available
- `Gi1/0/2` - second data port, and so on...

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

- `eth0` - management interface connected to the containerlab management network. Mapped to `GigabitEthernet0/0`.
- `eth1` - First data-plane interface. Mapped to `GigabitEthernet1/0/1` interface.
- `eth2` - Second data-plane interface. Mapped to `GigabitEthernet1/0/2` interface and so on.

/// note
Data interfaces may take 5+ minutes to function correctly after the node boots.
///

You must define interfaces in a contigous manner in your toplogy file. For example, if you want to use `Gi1/0/4` you must define `Gi1/0/1`, `Gi1/0/2` and `Gi1/0/3`. See the example below.

```yaml
name: my-cat9kv-lab
topology:
  nodes:
    cat9kv1:
      kind: cisco_cat9kv
      image: vrnetlab/vr-cat9kv:17.12.01p
    cat9kv2:
      kind: cisco_cat9kv
      image: vrnetlab/vr-cat9kv:17.12.01p

  links:
    - endpoints: ["cat9kv1:Gi1/0/1","cat9kv2:GigabitEthernet1/0/1"] 
    - endpoints: ["cat9kv1:Gi1/0/2","cat9kv2:GigabitEthernet1/0/2"]
    - endpoints: ["cat9kv1:Gi1/0/3", "cat9kv2:GigabitEthernet1/0/3"]
    - endpoints: ["cat9kv1:Gi1/0/4", "cat9kv2:GigabitEthernet1/0/4"]
```

/// warning
Regardless of how many links are defined in your containerlab topology, the Catalyst 9000v will always display 8 data-plane interfaces. Links/interfaces that you did not define in your containerlab topology will *not* pass any traffic.
///

When containerlab launches [[[ kind_display_name ]]] node the `GigabitEthernet0/0` interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `GigabitEthernet1/0/1+` need to be configured with IP addressing manually using CLI or other available management interfaces and will appear `unset` in the CLI:

```
c9kv(config-if)#do sh ip in br
Interface              IP-Address      OK? Method Status                Protocol
Vlan1                  unassigned      YES unset  administratively down down    
GigabitEthernet0/0     10.0.0.15       YES manual up                    up      
GigabitEthernet1/0/1   unassigned      YES unset  up                    up      
GigabitEthernet1/0/2   unassigned      YES unset  up                    up      
GigabitEthernet1/0/3   unassigned      YES unset  up                    up      
GigabitEthernet1/0/4   unassigned      YES unset  up                    up      
GigabitEthernet1/0/5   unassigned      YES unset  up                    up      
GigabitEthernet1/0/6   unassigned      YES unset  up                    up      
GigabitEthernet1/0/7   unassigned      YES unset  up                    up      
GigabitEthernet1/0/8   unassigned      YES unset  up                    up
```

## Features and options

### ASIC data-plane simulation configuration

The default ASIC simulation of the node will be UADP. To enable the Q200 simulation or to enable specific features for the UADP simulation, you must provide a `vswitch.xml` file (with the relevant configuration).

You can do this when building the image with [vrnetlab](../vrnetlab.md), Please refer to the README file in [vrnetlab/cat9kv](https://github.com/hellt/vrnetlab/blob/master/cat9kv/README.md) for more information.

You can also use supply the vswitch.xml file via `binds` in the containerlab topology file. Refer to the example below.

```yaml
name: my-cat9kv-lab
topology:
  nodes:
    node1:
      kind: cisco_cat9kv
      image: vrnetlab/vr-cat9kv:17.12.01p
    binds:
      - /path/to/vswitch.xml:/vswitch.xml
```

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
      image: vrnetlab/vr-cat9kv:17.12.01p
    env:
     VCPU: 6
     RAM: 20480
```
