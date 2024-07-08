---
search:
  boost: 4
kind_code_name: cisco_xrv9k
kind_display_name: Cisco XRv9k
---
# Cisco XRv9k

[Cisco XRv9k](https://www.cisco.com/c/en/us/products/collateral/routers/ios-xrv-9000-router/datasheet-c78-734034.html) virtualized router is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Cisco XRv9k nodes launched with containerlab come up pre-provisioned with SSH, SNMP, NETCONF and gNMI (if available) services enabled.

/// admonition | Resource requirements
    type: warning
XRv9k node is a resource hungry image. As of XRv9k 7.2.1 version the minimum resources should be set to 2vcpu/14GB. To be safe the defaults used in containerlab are 2vCPU/16G RAM.  
Image may take 25 minutes to fully boot, be patient. You can monitor the loading status with `docker logs -f <container-name>`.

If you need to tune the allocated resources, you can do so with setting `VCPU` and `RAM` environment variables for the node. For example, to set 4vcpu/16GB for the node:

```yaml
    iosxr:
      kind: cisco_xrv9k
      image: vr-xrv9k:7.10.1
      env:
        VCPU: 4
        RAM: 16384
```

///

## Managing Cisco XRv9k nodes

Cisco XRv9k node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Cisco XRv9k container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the XRv9kCLI
    ```bash
    ssh clab@<container-name/id>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh clab@<container-name> -p 830 -s netconf
    ```
=== "gNMI"
    using the best in class [gnmic](https://gnmic.kmrd.dev) gNMI client as an example:
    ```bash
    gnmic -a <container-name/node-mgmt-address> --insecure \
    -u clab -p clab@123 \
    capabilities
    ```

!!!info
    Default user credentials: `clab:clab@123`

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is:

- `GigabitEthernet0/0/0/X` or `Gi0/0/0/X`
- `TenGigabitEthernet0/0/0/X`, `TenGigE0/0/0X` or `Te0/0/0/X`

where X denotes the port number.

With that naming convention in mind:

- `Gi0/0/0/0` - first data port available
- `Gi0/0/0/1` - second data port, and so on...

/// note

1. Data port numbering starts at `0`.
2. Data interfaces may take 10+ minutes to come up, please be patient.
3. Cisco XRv9k can have up to 90 interfaces.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

- `eth0` - management interface connected to the containerlab management network.
- `eth1` - first data interface, mapped to the first data port of the VM (rendered as `Gi0/0/0/0`)
- `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `Gi0/0/0/1` and so on)

When containerlab launches [[[ kind_display_name ]]] node the management interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `Gi0/0/0/0+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

Cisco XRv9k nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `clab` user and management interfaces such as NETCONF, SNMP, gNMI.

#### Startup configuration

It is possible to make XRv9k nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: cisco_xrv9k
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.

## Lab examples

The following labs feature Cisco XRv9k node:

- [SR Linux and Cisco XRv9k](../../lab-examples/vr-xrv9k.md)
