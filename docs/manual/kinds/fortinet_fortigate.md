---
search:
  boost: 4
kind_code_name: fortinet_fortigate
kind_display_name: Fortinet Fortigate
---
# [[[ kind_display_name ]]]

[[[ kind_display_name ]]] virtualized security appliance is identified with the `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using the [hellt/vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

The integration of [[[ kind_display_name ]]] has been tested with v7.0.14 release. Note, that releases >= 7.2.0 would require a valid license and internet access to activate the [[[ kind_display_name ]]] VM.

## Getting [[[ kind_display_name ]]] disk image

Users can obtain the qcow2 disk image for [[[ kind_display_name ]]] VM from the [official support site](https://support.fortinet.com/Download/VMImages.aspx); a free account required. Download the "New deployment" variant of the FGVM64 VM for the KVM platform.

Extract the downloaded zip file and rename the `fortios.qcow2` to `fortios-vX.Y.Z.qcow2` where `X.Y.Z` is the version of the Fortigate VM. Put the renamed file in the `fortigate` directory of the cloned [hellt/vrnetlab](https://github.com/hellt/vrnetlab) project and run `make` to build the container image.

## Managing [[[ kind_display_name ]]] nodes

/// note
Containers with [[[ kind_display_name ]]] VM inside will take ~2min to fully boot.  
You can monitor the progress with the `docker logs -f <container-name>` command.
///

[[[ kind_display_name ]]] node launched with containerlab can be managed via the following interfaces:

/// tab | bash
to connect to a `bash` shell of a running fortigate container:

```bash
docker exec -it <container-name/id> bash
```

///
/// tab | CLI
to connect to the Fortigate CLI

```bash
ssh admin@<container-name/id/IP-addr>
```

///
/// tab | Web UI (HTTP)
Fortigate VM comes with HTTP(S) server with a GUI manager app. You can access the Web UI using http schema.

```bash
http://<container-name/id/IP-addr>
```

You can expose container's port 80 with the [`ports`](../nodes.md#ports) setting in containerlab and get access to the Web UI using your containerlab host IP.
///
/// note
Default login credentials: `admin:admin`
///

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `portX`, where `X` is the port number.

With that naming convention in mind:

* `port2` - first data port available
* `port3` - second data port, and so on...

/// admonition
    type: warning
Data port numbering starts at `2`, as `port1` is reserved for management connectivity. Attempting to use `port1` in a containerlab topology will result in an error.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth0` - management interface connected to the containerlab management network (rendered as `port1` in the CLI)
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `port2`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `port3` and so on)

When containerlab launches [[[ kind_display_name ]]] node the `port1` interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `port2+` need to be configured with IP addressing manually using CLI or other available management interfaces.
