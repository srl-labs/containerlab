---
search:
  boost: 4
---
# RARE/freeRtr

[RARE](http://rare.freertr.org) stands for Router for Academia, Research & Education. It is an open source routing platform, used to create a network operating system (NOS) on commodity hardware (a white box switch). RARE uses FreeRtr as a control plane software  and is thus often referred to as RARE/freeRtr.

???info "what is RARE?"
    RARE/freeRtr has the particularity to run interchangeably different dataplanes such P4 INTEL TOFINO, P4 BMv2, DPDK, XDP, libpcap or UNIX UDP raw sockets. This inherent property allows RARE/freeRtr to run multiple use cases requiring different bandwidth capability.

    It can be used as:

    * a full featured versatile DPDK [SOHO](https://wiki.geant.org/x/JK7TC) router able to handle nx1GE, nx10GE and a couple of 100GE.
    * a service provider Metropolitan Arean Network [IPv4/IPv6 MPLS router](https://wiki.geant.org/x/hLDTC)
    * a full featured [BGP Route Reflector](https://wiki.geant.org/x/q5rTC)

    More information [here](http://docs.freertr.org) and [here](http://rare.freertr.org).

RARE/freeRtr: is a container image that uses `linux` kind to run RARE/freeRtr.

## Getting RARE image

RARE/freeRtr container image is available to everyone on [GitHub Container Registry](https://ghcr.io/rare-freertr/freertr-containerlab)

The container image above is a nightly built of [RARE/freeRtr](https://github.com/rare-freertr/freeRtr) control plane `master` branch.

<!-- ```yaml
name: rtr000

topology:
  nodes:
    rtr1:
      kind: linux
      image: ghcr.io/rare-freertr/freertr-containerlab:latest 
      binds: 
        - __clabNodeDir__/run:/rtr/run
    rtr2:
      kind: linux
      image: ghcr.io/rare-freertr/freertr-containerlab:latest 
      binds: 
        - __clabNodeDir__/run:/rtr/run
  links:
    - endpoints: ["rtr1:eth1","rtr2:eth1"]
```

!!!warning
    Create beforehand RARE/freeRtr `__clabNodeDir__/run` folder where all router artefacts are located.
    ```
    mkdir __clabNodeDir__/run
    ``` -->

## Managing RARE/freeRtr nodes

RARE/freeRtr node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running RARE/freeRtr container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
    to access RARE/freeRtr CLI
    ```
    telnet localhost 2323
    ```
=== "CLI"
    to connect to RARE/freeRtr CLI directly
    ```telnet
    telnet <container-name/id@eth0>
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

RARE/freeRtr nodes have a dedicated `__clabNodeDir__/run` directory that is used to persist the configuration of the node.

#### Default node configuration

RARE/freeRtr configuration consist in 2 files:

* `rtr-hw.txt` also called freeRtr hardware file
* `rtr-sw.txt` also called freeRtr software file

This is located into:

* `__clabNodeDir__/run/conf` at the containerlab host server
* `/rtr/run/conf` inside the container

#### User defined config

It is possible to make RARE/freeRtr nodes to boot up with a user-defined config instead of a built-in one. In this case you'd have to put `rtr-hw.txt` and `rtr-sw.txt` files into `__clabNodeDir__/run`  

#### Saving configuration

Configuration is saved using `write` command using RARE/freeRtr CLI. The router configuration will be saved at `__clabNodeDir__/run/conf/rtr-sw.txt`

### License

As an open source software, RARE/freeRtr does not require any license file.

## Build RARE/freeRtr Container

RARE/freeRTr container can be built:

```
git clone https://github.com/rare-freertr/freeRtr-containerlab.git
cd freeRtr-containerlab
docker build --no-cache -t freertr-containerlab:latest .
```

### File mounts

As previously mentioned it is necessary to create `run` folder for each routers.

In the lab example above:

```yaml
cd freeRtr-containerlab
mkdir ./clab-rtr000/rtr1/run
mkdir ./clab-rtr000/rtr1/run 
```

```bash
containerlab deploy --topo rtr000.clab.yml
INFO[0000] Containerlab v0.38.0 started
INFO[0000] Parsing & checking topology file: rtr000.clab.yml
INFO[0000] Creating lab directory: /root/development/testclab/freeRtr-containerlab/clab-rtr000
INFO[0001] Creating docker network: Name="clab", IPv4Subnet="172.20.20.0/24", IPv6Subnet="2001:172:20:20::/64", MTU="1500"
INFO[0002] Creating container: "rtr2"
INFO[0006] Creating container: "rtr1"
INFO[0007] Creating virtual wire: rtr1:eth1 <--> rtr2:eth1
INFO[0007] Adding containerlab host entries to /etc/hosts file
+---+------------------+--------------+--------------------------------------------------+-------+---------+----------------+----------------------+
| # |       Name       | Container ID |                      Image                       | Kind  |  State  |  IPv4 Address  |     IPv6 Address     |
+---+------------------+--------------+--------------------------------------------------+-------+---------+----------------+----------------------+
| 1 | clab-rtr000-rtr1 | ff666c777f68 | ghcr.io/rare-freertr/freertr-containerlab:latest | linux | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
| 2 | clab-rtr000-rtr2 | 873ec955d2b9 | ghcr.io/rare-freertr/freertr-containerlab:latest | linux | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+------------------+--------------+--------------------------------------------------+-------+---------+----------------+----------------------+
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

* [RARE/freeRtr hello world](../../lab-examples/rare-freertr-000.md)
