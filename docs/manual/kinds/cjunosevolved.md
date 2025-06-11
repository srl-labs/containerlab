---
search:
  boost: 4
kind_code_name: juniper_cjunosevolved
kind_display_name: Juniper cJunosEvolved
---
# Juniper cJunosEvolved

[Juniper cJunosEvolved](https://www.juniper.net/documentation/product/us/en/cjunosevolved/) is a containerized Junos OS Evolved router identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is a KVM based container that can emulate either of these PTX platforms:

* `PTX10002-36QDD`- A fixed form factor 800G transport router based on Juniper's Express 5 (aka BX) ASIC.
* `PTX10001-36MR` - A fixed form factor 400G transport router based on Juniper's Express 4 (aka BT) ASIC.

The above platforms are selected via these environmental variables being specified in the topology YAML file as shown in the `lab-examples` directories for cjunosevolved:

 `CPTX_COSIM: "BX"`- For PTX10002-36QDD
 `CPTX_COSIM: "BT"`- For PTX10001-36MR

Juniper cJunosEvolved nodes launched with containerlab can be provisioned to enable SSH, SNMP, NETCONF and gNMI services.

## How to obtain the image

The container image can be freely downloaded from the [Juniper support portal](https://support.juniper.net/support/downloads/?p=cjunos-evolved) without a Juniper account. Type cJunosEvolved in the `Find a Product` search box.

## Managing Juniper cJunosEvolved nodes

!!!note
    Containers with cJunosEvolved inside will take ~5min to boot to login prompt.
    You can monitor the progress with `docker logs -f <container-name>`.
!!!note
    The management port IP is assigned by containerlab and is merged into the cJunosEvolved startup config.
    Please refer to `docs/lab-examples/srl-cjunosevolved.md` for an example.

=== "CLI via SSH"
    to connect to the cJunosEvolved CLI
    ```bash
    ssh admin@<container-name>
    ```

=== "Enter CLI directly"
    cJunosEvolved has to be fully booted before this succeeds
    ```bash
    docker exec -ti <container-name> cli
    ```

=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh admin@<container-name> -p 830 -s netconf
    ```

!!!info
    Default user credentials: `admin:admin@123`

## Interface naming

You can refer to the following document for details of the interface mapping (https://www.juniper.net/documentation/product/us/en/cjunosevolved/)

The default unchannelized interface mode is described here. This provides 36 interfaces for BX and 12 for BT flavors of cJunosEvolved. The channelized mode provides 144 interfaces for BX and 72 for BT. The mapping for all of these is described in the Juniper deployment document referenced above.

The Linux host side interfaces are mapped to the JunosEvolved CLI notation as described in the document. To summarize:

Juniper cJunosEvolved Linux uses the following mapping rules:

* `eth0`- management interface connected to the containerlab management network
* `eth1 -eth3` - Reserved interfaces  **Do not use these**
* `eth4 onwards` - WAN interfaces.

* For the BX unchannelized mode:

The Linux `eth4 – eth39` interfaces correspond to the `et-0/0/0 – et-0/0/35` interfaces in the JunosEvolved CLI configuration.

* For the BT unchannelized mode:

The Linux `eth4 – eth15` interfaces correspond to the `et-0/0/0 – et-0/0/11` interfaces in the JunosEvolved CLI configuration.

When containerlab launches -{{ kind_display_name }}- it assigns an IP address to the container's `eth0` management interface.
This interface is transparently stitched with the cJunosEvolved's `re0:mgmt-0` interface such that users can reach the management plane of the -{{ kind_display_name }}- using containerlab's assigned IP.

The WAN interfaces need to be configured with IP addressing manually using CLI or other available management interfaces.
You could also pass in a startup CLI configuration file that has the WAN interface addresses specified. For example,
refer to `lab-examples/srlcjunosevo01/cjunosevo.cfg`.

## Features and options

The user can configure cJunosEvolved in either the BX or BT mode via providing an env variable in the containerlab YAML file.
Refer to `lab-examples/srlcjunosevo/srlcjunosevo01.clab.yml`.

`CPTX_COSIM: "BX"` provisions the BX mode, i.e. the `PTX10002-36QDD` platform.
`CPTX_COSIM: "BT"` provisions the BT mode, i.e. the `PTX10001-36MR` platform.

### Node configuration

Juniper cJunosEvolved nodes come up with a basic configuration. Users, management interfaces, and protocols such as SSH, NETCONF and SNMP are configured.

#### Startup configuration

You can make cJunosEvolved nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: juniper_cjunosevolved
      startup-config: cjunosevo.cfg
```

With this knob containerlab is instructed to take a file `cjunosevo.cfg` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.

## Lab examples

The following labs feature the Juniper cJunosEvolved node:

* [SR Linux and Juniper cJunosEvolved](../../lab-examples/srl-cjunosevolved.md)

## Known issues and limitations

* To check the boot log, use `docker logs -f <node-name>`
