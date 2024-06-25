---
search:
  boost: 4
---
# Cisco XRd

[Cisco XRd](https://www.cisco.com/c/en/us/support/routers/ios-xrd/series.html) Network OS is identified with `xrd` or `cisco_xrd` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a node.

XRd comes in two [variants](https://xrdocs.io/virtual-routing/tutorials/2022-08-22-xrd-images-where-can-one-get-them/#xrd-form-factors):

* control-plane
* vrouter

Containerlab supports only the control-plane flavor of XRd, as it allows to build topologies using virtual interfaces, whereas vrouter requires PCI interfaces to be attached to it.

!!!tip
    Consult with [XRd Tutorials](https://xrdocs.io/virtual-routing/tutorials/2022-08-22-xrd-images-where-can-one-get-them/) series to get an in-depth understanding of XRd requirements and capabilities.

## Getting XRd

XRd image is available for download only for users who have an active service account[^1].

## Host server requirements

You should increase the value of `user.max_inotify_instances`:

```bash
sysctl -w user.max_inotify_instances=64000
```

## Managing XRd nodes

There are several management interfaces supported by XRd nodes:

=== "CLI"
    to connect to a XR CLI shell of a running XRd container:
    ```bash
    docker exec -it <container-name/id> /pkg/bin/xr_cli.sh
    ```
=== "bash"
    to connect to a `bash` shell of a running XRd container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "SSH"
    `ssh clab@<container-name>`  
    Password: `clab@123`
=== "gNMI"
    gNMI server runs on `57400` port in the insecure mode (no TLS).  
    Using [gnmic](https://gnmic.openconfig.net) gNMI client as an example:
    ```bash
    gnmic -a <container-name/node-mgmt-address>:57400 --insecure \
      -u clab -p clab@123 \
      capabilities
    ```
=== "Netconf"
    Netconf server runs on `830` port:  
    ```
    ssh clab@<container-name> -p 830 -s netconf
    ```

!!!info
    Default credentials: `clab:clab@123`

## Interfaces mapping

XRd container uses the following mapping for its Linux interfaces[^2]:

* `eth0` - management interface connected to the containerlab management network
* `Gi0-0-0-0` - first data interface mapped to `Gi0/0/0/0` internal interface.
* `Gi0-0-0-N` - Nth data interface mapped to `Gi0/0/0/N` internal interface.

When containerlab launches XRd node, it will set IPv4/6 addresses as assigned by docker to the `eth0` interface and XRd node will boot with these addresses configured for its `MgmtEth0`. Data interfaces `Gi0/0/0/N` need to be configured with IP addressing manually.

```
RP/0/RP0/CPU0:xrd#sh ip int br
Wed Dec 21 12:04:13.049 UTC

Interface                      IP-Address      Status          Protocol Vrf-Name
MgmtEth0/RP0/CPU0/0            172.20.20.5     Up              Up       default
```

## Features and options

### Node configuration

XRd nodes have a dedicated [`config`](../conf-artifacts.md#identifying-a-lab-directory) directory that is used to persist the configuration of the node and expose internal directories of the NOS.

For XRd nodes, containerlab exposes the following file layout of the node's lab directory:

* `xr-storage` (dir): a directory that is mounted to `/xr-storage` path of the NOS and is used to persist changes made to the node as well as provides access to the logs and various runtime files.
* `first-boot.cfg` - a configuration file in Cisco IOS-XR CLI format that the node boots with.

#### Default node configuration

It is possible to launch nodes of `cisco_xrd` kind with a basic config or to provide a custom config file that will be used as a startup config instead.

When a node is defined without `startup-config` statement present, containerlab will generate an empty config from [this template](https://github.com/srl-labs/containerlab/blob/main/nodes/xrd/xrd.cfg) and copy it to the config directory of the node.

```yaml
# example of a topo file that does not define a custom config
# as a result, the config will be generated from a template
# and used by this node
name: xrd
topology:
  nodes:
    xrd:
      kind: cisco_xrd
```

#### User defined config

It is possible to make XRd nodes to boot up with a user-defined config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property a user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
name: xrd
topology:
  nodes:
    xrd:
      kind: cisco_xrd
      startup-config: xrd.cfg
```

When a config file is passed via `startup-config` parameter it will be used during an initial lab deployment. However, a config file that might be in the lab directory of a node takes precedence over the startup-config[^3].

With such topology file containerlab is instructed to take a file `xrd.cfg` from the current working directory and copy it to the lab directory for that specific node under the `/first-boot.cfg` name. This will result in this config acting as a startup-config for the node.

To provide a user-defined config, take the [default configuration template](https://github.com/srl-labs/containerlab/blob/main/nodes/xrd/xrd.cfg) and add the necessary configuration commands without changing the rest of the file. This will result in proper automatic assignment of IP addresses to the management interface, as well as applying user-defined commands.

!!!tip
    Check [SR Linux and XRd](../../lab-examples/srl-xrd.md) lab example where startup configuration files are provided to both nodes to see it in action.

#### Configuration persistency

XRd nodes persist their configuration in `<lab-directory>/<node-name>/xr-storage` directory. When a user commits changes to XRd nodes using one of the management interfaces, they are kept in the configuration DB (but not exposed as a configuration file).

This capability allows users to configure the XRd node, commit the changes, then destroy the lab (without using `--cleanup` flag to keep the lab dir intact) and on a subsequent deploy action, the node will boot with the previously saved configuration.

## Known issues and limitations

Note, that XRd requires elevated number of inotify resources. If you happen to see errors in the xrd bootlog about inotify resources, consult with [this article](https://xrdocs.io/virtual-routing/tutorials/2022-08-22-setting-up-host-environment-to-run-xrd/#inotify-max-user-watches-and-inotify-max-user-instances-settings) on how to increase them.

## Lab examples

The following labs feature XRd nodes:

* [SR Linux and XRd](../../lab-examples/srl-xrd.md)

[^1]: https://xrdocs.io/virtual-routing/tutorials/2022-08-22-xrd-images-where-can-one-get-them/
[^2]: It is not yet possible to manually assign interface mapping rules in containerlab for XRd nodes. PRs are welcome.
[^3]: if startup config needs to be enforced, either deploy a lab with `--reconfigure` flag, or use [`enforce-startup-config`](../nodes.md#enforce-startup-config) setting.
