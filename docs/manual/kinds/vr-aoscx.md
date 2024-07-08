---
search:
  boost: 4
kind_code_name: aruba_aoscx
kind_display_name: ArubaOS-CX
---
# Aruba ArubaOS-CX

ArubaOS-CX virtualized switch is identified with `aruba_aoscx` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Managing vr-aoscx nodes

!!!note
    Containers with AOS-CX inside will take ~2min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Aruba AOS-CX node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-aoscx container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the AOS-CX CLI (password `admin`)
    ```bash
    ssh admin@<container-name/id>
    ```

!!!info
    Default user credentials: `admin:admin`

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `1/1/X`, where `X` is the port number.

With that naming convention in mind:

* `1/1/1` - first data port available
* `1/1/2` - second data port, and so on...

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `1/1/1`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `1/1/2` and so on)

When containerlab launches [[[ kind_display_name ]]] node the `1/1/1` interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `1/1/2+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

ArubaOS-CX nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `admin` user with the provided password.

#### Startup configuration

It is possible to make ArubaOS-CX nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: aruba_aoscx
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.
