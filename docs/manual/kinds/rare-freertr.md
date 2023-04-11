---
search:
  boost: 4
---
# RARE/freeRtr

[RARE](http://rare.freertr.org) stands for Router for Academia, Research & Education. It is an open source routing platform, used to create a network operating system (NOS) on commodity hardware (a white box switch). RARE uses FreeRtr as a control plane software  and is thus often referred to as RARE/freeRtr.

RARE nodes are identified by the `rare` kind in the [topology file](../topo-def-file.md).

???info "what is RARE?"
    RARE/freeRtr has the particularity to run interchangeably different dataplanes such P4 INTEL TOFINO, P4 BMv2, DPDK, XDP, libpcap or UNIX UDP raw sockets. This inherent property allows RARE/freeRtr to run multiple use cases requiring different bandwidth capability.

    It can be used as:

    * a full featured versatile DPDK [SOHO](https://wiki.geant.org/x/JK7TC) router able to handle nx1GE, nx10GE and a couple of 100GE.
    * a service provider Metropolitan Arean Network [IPv4/IPv6 MPLS router](https://wiki.geant.org/x/hLDTC)
    * a full featured [BGP Route Reflector](https://wiki.geant.org/x/q5rTC)

    More information [here](http://docs.freertr.org) and [here](http://rare.freertr.org).

## Getting RARE image

RARE/freeRtr container image is freely available on [GitHub Container Registry](https://ghcr.io/rare-freertr/freertr-containerlab).

The container image is nightly build of a [RARE/freeRtr](https://github.com/rare-freertr/freeRtr) control plane off the `master` branch.

## Managing RARE/freeRtr nodes

RARE/freeRtr node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running RARE/freeRtr container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to RARE/freeRtr CLI directly
    ```telnet
    telnet <container-name/id>
    ```

!!!info
    Default user credentials: `rare:rare`

## Interfaces mapping

RARE/freeRtr container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface
* `eth2` - second data interface
* `eth<n>` - n<sup>th</sup> data interface

When containerlab launches RARE/freeRtr node:

* It will assign IPv4/6 address to the `eth0` interface.
* Data interface `eth1`, `eth2`, `eth<n>` need to be configured with IP addressing manually.

## Features and options

### Node configuration

RARE/freeRtr nodes have a dedicated `/run`[^1] directory that is used to persist the configuration of the node which consists of 2 files:

* `rtr-hw.txt` also called freeRtr hardware file
* `rtr-sw.txt` also called freeRtr software file

#### User defined config

It is possible to make RARE/freeRtr nodes to boot up with a user-defined config instead of a default one. In this case you'd have to create `rtr-hw.txt` and `rtr-sw.txt` files and bind mount them to the `/rtr/run/conf` dir:

```yaml
nodes:
  rtr1:
    kind: rare
    image: ghcr.io/rare-freertr/freertr-containerlab:latest
    binds:
      - rtr-hw.txt:/rtr/run/conf/rtr-hw.txt
      - rtr-sw.txt:/rtr/run/conf/rtr-sw.txt
```

#### Saving configuration

Configuration is saved using `write` command using RARE/freeRtr CLI. The router configuration will be saved at `<lab_dir>/<node_name>/run/`

### License

As an open source software, RARE/freeRtr does not require any license file.

## Build RARE/freeRtr Container

RARE/freeRTr container can be built:

```bash
git clone https://github.com/rare-freertr/freeRtr-containerlab.git
cd freeRtr-containerlab
docker build --no-cache -t freertr-containerlab:latest .
```

### File mounts

During lab initialisation, each node will have their `run` folder created.

In the lab example above:

```bash
cd freeRtr-containerlab
$ ls clab-rtr000/rtr1/run/
conf  logs  mrt  ntfw  pcap

$ ls clab-rtr000/rtr2/run/
conf  logs  mrt  ntfw  pcap
```

```bash
root@debian:~/development/testclab/freeRtr-containerlab# tree clab-rtr000/
clab-rtr000/
├── ansible-inventory.yml
├── authorized_keys
├── rtr1
│   └── run
│       ├── conf
│       │   ├── hwdet-all.sh
│       │   ├── hwdet.eth
│       │   ├── hwdet.mac
│       │   ├── hwdet-main.sh
│       │   ├── hwdet.ser
│       │   ├── pcapInt.bin -> /rtr/pcapInt.bin
│       │   ├── rtr-hw.txt
│       │   └── rtr-sw.txt
│       ├── logs
│       │   └── freertr.log
│       ├── mrt
│       ├── ntfw
│       └── pcap
├── rtr2
│   └── run
│       ├── conf
│       │   ├── hwdet-all.sh
│       │   ├── hwdet.eth
│       │   ├── hwdet.mac
│       │   ├── hwdet-main.sh
│       │   ├── hwdet.ser
│       │   ├── pcapInt.bin -> /rtr/pcapInt.bin
│       │   ├── rtr-hw.txt
│       │   └── rtr-sw.txt
│       ├── logs
│       │   └── freertr.log
│       ├── mrt
│       ├── ntfw
│       └── pcap
└── topology-data.json

```

* `conf` folder is where RARE/freeRtr configuration files are located
* `logs` folder is where RARE/freeRtr logs files are located (output of `show logging`)
* `pcap` folder is where `pcap` files are located (`packet capture eth1`)
* `ntfw` folder is where netflow files are stored (future use - not configured currently in default config)
* `mrt` folder is where `bmp` output files are stored (future use - not configured currently in default config)

## Lab example

The following labs feature RARE/freeRtr node:

* [RARE/freeRtr](../../lab-examples/rare-freertr.md)

[^1]: `/run` directory is created in the [Lab directory](../conf-artifacts.md#identifying-a-lab-directory) for each RARE/freeRtr node.
