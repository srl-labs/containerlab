# Nokia SR OS

[Nokia SR OS](https://www.juniper.net/documentation/us/en/software/vr-sros/vr-sros-deployment/topics/concept/understanding-vr-sros.html) virtualized router is identified with `vr-sros` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

vr-sros nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing vr-sros nodes

!!!note
    Containers with SR OS inside will take ~3min to fully boot.  
    You can monitor the progress with `watch docker ps` waiting till the status will change to `healthy`.

Nokia SR OS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-sros container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the SR OS CLI
    ```bash
    ssh admin@<container-name/id>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh root@<container-name> -p 830 -s netconf
    ```
=== "gNMI"
    using the best in class [gnmic](https://gnmic.kmrd.dev) gNMI client as an example:
    ```bash
    gnmic -a <container-name/node-mgmt-address> --insecure \
    -u admin -p admin \
    capabilities
    ```

!!!info
    Default user credentials: `admin:admin`

## Interfaces mapping
vr-sros container uses the following mapping for its interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of SR OS line card
* `eth2+` - second and subsequent data interface

???warning "data interfaces need to be defined consequently"
    Note, that data interfaces need to be defined in the topology file in a consequent fashion. Meaning that no gaps are allowed between the interfaces definition:
    ```yaml
    links:
      - endpoints: ["srl:e1-1", "sros:eth1"]
      # this is not allowed by vr-sros kind.
      # if eth5 is needed, define eth2-4 as well
      - endpoints: ["srl:e1-5", "sros:eth5"]
    ```

When containerlab launches vr-sros node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Variants
Virtual SR OS simulator can be run in multiple HW variants as explained in [the vSIM installation guide](https://documentation.nokia.com/cgi-bin/dbaccessfilename.cgi/3HE15836AAADTQZZA01_V1_vSIM%20Installation%20and%20Setup%20Guide%2020.10.R1.pdf).

`vr-sros` container images come with [pre-packaged SR OS variants](https://github.com/hellt/vrnetlab/tree/master/sros#variants) as defined in the upstream repo as well as support [custom variant definition](https://github.com/hellt/vrnetlab/tree/master/sros#custom-variant). The pre-packaged variants are identified by the variant name and come up with cards and mda already configured. Custom variants, on the other hand, give users the full flexibility in emulated hardware configuration, but cards and mdas would need to be configured manually.

To make vr-sros to boot in one of the pacakged variants use its name like that:
```yaml
topology:
  nodes:
    sros:
      kind: vr-sros
      image: vrnetlab/vr-sros:20.10.R1
      type: sr-1s # if type omitted, the default sr-1 variant will be used
      license: license-sros20.txt
```

Custom variant can be defined as simple as that:
```yaml
# for distributed chassis CPM and IOM are indicated with markers cp: and lc:
# notice the delimeter string `___` that MUST be present between CPM and IOM portions
# max_nics value is provided in `lc` part.
# mem is provided in GB
# quote the string value
type: "cp: cpu=2 ram=4 chassis=ixr-e slot=A card=cpm-ixr-e ___ lc: cpu=2 ram=4 max_nics=34 chassis=ixr-e slot=1 card=imm24-sfp++8-sfp28+2-qsfp28 mda/1=m24-sfp++8-sfp28+2-qsfp28"
```

```yaml
# an integrated custom type definition
# note, no `cp:` marker is needed
type: "slot=A chassis=ixr-r6 card=cpiom-ixr-r6 mda/1=m6-10g-sfp++4-25g-sfp28"
```

### Node configuration
vr-sros nodes come up with a basic "blank" configuration where only the card/mda are provisioned, as well as the management interfaces such as Netconf, SNMP, gNMI.

#### User defined config
It is possible to make SR OS nodes to boot up with a user-defined config instead of a built-in one. With a [`config`](../nodes.md#config) property of the node/kind a user sets the path to the config file that will be mounted to a container and used as a startup config:

```yaml
name: sros_lab
topology:
  nodes:
    sros:
      kind: vr-sros
      config: myconfig.txt
```

With such topology file containerlab is instructed to take a file `myconfig.txt` from the current working directory, copy it to the lab directory for that specific node under the `/tftpboot/config.txt` name and mount that dir to the container. This will result in this config to act as a startup config for the node.

### License
Path to a valid license must be provided for all vr-sros nodes with a [`license`](../nodes.md#license) directive.

### File mounts
When a user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For `vr-sros` kind containerlab creates `tftpboot` directory where the license file will be copied.

## Lab examples
The following labs feature vr-sros node:

- [SR Linux and vr-sros](../../lab-examples/vr-sros.md)