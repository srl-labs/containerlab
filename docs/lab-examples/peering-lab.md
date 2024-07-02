---
comments: true
---
# Peering lab

Internet eXchange Points are the glue that connects the Internet. They are the physical locations where ISPs, CDNs and all other ASN holders connect to exchange traffic. While traffic exchange might sound simple, it is a complex process with lots of moving parts:

* Peering routers configuration.
* Route Servers configuration.
* Route filtering.
* MANRS compliance.
* RPKI validation.
* IXP services enablement.

Each of these topics is a whole body of knowledge on its own and various Internet exchange consortiums have published best practices and guidelines to help IXP operators and their members to configure their networks properly.

The guidelines and current best practices are best to be reinforced in a lab environment. And with this thought in mind, we present containerlab users with this hands-on lab simulating an IXP with Route Servers and peering members.

## Lab summary

| Summary                   |                                                                                                                                                              |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Lab name**              | Peering Lab                                                                                                                                                  |
| **Lab components**        | [Nokia SR OS][nokia-sros], [FRRouting (FRR)][frr], [OpenBGPd][openbgpd] and [BIRD][bird] route servers                                                       |
| **Resource requirements** | :fontawesome-solid-microchip: 2 vCPU <br/>:fontawesome-solid-memory: 6 GB                                                                                    |
| **Lab**                   | [hellt/sros-frr-ixp-lab][lab]                                                                                                                                |
| **Version information**   | [`containerlab:0.41.1`][clab-install], `Nokia SR OS:23.3.R1`, [`FRR:8.4.1`][frr-container], [`BIRD:2.13`][bird-container], [`openbgpd:7.9`][obgpd-container] |
| **Authors**               | Roman Dodin [:material-twitter:][rd-twitter] [:material-linkedin:][rd-linkedin]                                                                              |

## Prerequisites

Since containerlab uses containers as the nodes of a lab, the Docker engine has to be [installed](../install.md#pre-requisites) on the host system first.

## Lab topology

The lab topology used in this lab needs to be flexible enough to practice various IXP scenarios, yet simple enough to be easily understood and configured. The following topology was chosen for this lab:

![pic1-topo](https://gitlab.com/rdodin/pics/-/wikis/uploads/f8d11f77fdacb03bce904f9abd37b587/image.png){.img-shadow}

On a physical level, the lab topology consists of two IXP members running Nokia SR OS and FRR network OSes accordingly. Each peer has a single interface connected to the IXP network (aka Peering LAN) to peer with other members and exchange traffic.

Our IXP lab also features a redundant pair of Route Servers powered by two most commonly used open-source routing daemons: OpenBGPd and BIRD. The Route Servers are connected to the IXP network via a dedicated `eth1` interface.

From the control-plane perspective, two peers establish eBGP sessions with the Route Servers and announce their networks. To keep things simple, each peer announces only one network, namely peer1 announces `10.0.0.1/32` and peer2 - `10.0.0.2/32`.

![pic2-topo](https://gitlab.com/rdodin/pics/-/wikis/uploads/9d09bf1a777394653d06af98cce37e53/image.png){.img-shadow}

The Route Servers receive NLRIs from peers and pass them over to the other IXP members.

## Obtaining container images

Every component of this lab is openly available and can be downloaded from public repositories, but Nokia SR OS, which has to be obtained from Nokia representatives. In this lab SR OS container image name is `sros:23.3.R1`.

## Topology definition

The topology definition file for this lab is available in the [lab repository][lab]. The topology file - [`ixp.clab.yml`][lab-file] - declaratively describes the lab topology and is used by containerlab to create the lab environment.

### Peers

The two IXP members are defined as follows:

```yaml
topology:
  nodes:
    peer1:
      kind: vr-nokia_sros
      image: sros:23.3.R1 #(1)!
      license: license.key
      startup-config: configs/sros.partial.cfg

    peer2:
      kind: linux
      image: quay.io/frrouting/frr:8.4.1
      binds:
        - configs/frr.conf:/etc/frr/frr.conf
        - configs/frr-daemons.cfg:/etc/frr/daemons
```

1. SR OS container has to be requested from Nokia or built manually from qcow2 disk image using `hellt/vrnetlab` project as explained [here](../manual/vrnetlab.md).

Apart from typical containerlab node definitions statements like `kind` and `image`, for SR OS we leverage the [`vr-nokia_sros`](../manual/kinds/vr-sros.md) kind, `license` and `startup-config` keys to provide the SR OS container with the license key and the startup configuration file respectively. Check out [Basic configuration](#basic-configuration) section for more details on the contents of startup-configuration files for each topology member.

For the FRR, which uses the public official container image, node we leverage the `binds` key to mount the FRR configuration file and the FRR daemon configuration file into the container. Again, the contents of these files are explained in the [Basic configuration](#basic-configuration) section.

### Route Servers

Both route servers are based on the [`linux`](../manual/kinds/linux.md) kind which represents a regular linux container:

```yaml
rs1: # OpenBGPd route server
  kind: linux
  image: quay.io/openbgpd/openbgpd:7.9
  binds:
    - configs/openbgpd.conf:/etc/bgpd/bgpd.conf
  exec:
    - "ip address add dev eth1 192.168.0.3/24"

rs2: # BIRD route server
  kind: linux
  image: ghcr.io/srl-labs/bird:2.13
  binds:
    - configs/bird.conf:/etc/bird.conf
  exec:
    - "ip address add dev eth1 192.168.0.4/24"
```

OpenBGPd server uses an [official container image][obgpd-container] and mounts the OpenBGPd configuration file into the container.

BIRD doesn't have an official container image, so we [created](https://github.com/srl-labs/bird-container) a BIRD v2.13 container image published at [ghcr.io/srl-labs/bird][bird-container][^1] and also mount the BIRD configuration file into the container.

### Peering LAN and links

At the geographical center of our topology lies the Peering LAN, which is represented by the `ixp-net` node that uses the [`bridge`](../manual/kinds/bridge.md) kind.

```yaml
ixp-net:
  kind: bridge
```

As per the kind's documentation, the `bridge` kind uses an underlying linux bridge and allows topology nodes to connect their interface to the bridge. Note, that the linux bridge with the matching name (which is `ixp-net` in our case) must be created on the host before containerlab starts the lab environment.

Finally, we define the links between the nodes. As per our topology, each peer and each route server has a single interface connected to the Peering LAN. The links are defined as follows:

```yaml
links:
  - endpoints: ["peer1:eth1", "ixp-net:port1"]
  - endpoints: ["peer2:eth1", "ixp-net:port2"]
  - endpoints: ["rs1:eth1", "ixp-net:port3"]
  - endpoints: ["rs2:eth1", "ixp-net:port4"]
```

With the links defined, our connectivity diagram looks like this:

![pic3-topo](https://gitlab.com/rdodin/pics/-/wikis/uploads/6cdfce818b4a9d51ba7d3d938dfd961a/image.png){.img-shadow}

## Basic configuration

Using [`startup-config`](../manual/nodes.md#startup-config) and [`binds`](../manual/nodes.md#binds) configuration setting every node in our lab is equipped with the startup configuration that makes it possible to deploy a functioning IXP environment.

The basic configuration that is captured in the startup configuration files contains the following:

* basic interface configuration for each node
* basic BGP configuration for the peer nodes to enable peering with the route servers
* basic import/export policies on the peer's side
* basic Route Server configuration with no filtering or route validation

### SR OS

Nokia SR OS startup configuration file is provided in the form of a CLI-styled configuration blob that is captured in the [sros-partial.cfg][sros-partial-cfg]. The statements in this config file are applied when the node is started and ready to accept CLI commands.

!!!tip
    Throughout this lab we will introduce and explain different BGP features and provide the relevant configuration snippets. As it takes time to put in writing all the features, ref links, and stories around them, we created a configuration cheat sheet that contains a condensed version of the BGP peering configuration snippets for SR OS. The cheat sheet is available at [sajusal/sros-peering](https://github.com/sajusal/sros-peering/blob/main/README.md) repository and should help you to get started with the lab in a self-exploration mode.

### FRR

FRR configuration is split between the two files:

* [frr.conf][frr-conf-basic] - contains the basic FRR configuration, which includes the most simple BGP configuration to enable peering.
* [daemons.cfg][frr-daemons-basic] - contains the list of FRR daemons to be started

### Route Servers

Router servers are no different and equipped with the basic route server configuration that enables unfiltered peering. The configuration files are bind mounted to each respective container as follows:

* OpenBGPd - [bgpd.conf][obgpd-conf-basic]
* BIRD - [bird.conf][bird-conf-basic]

## Lab lifecycle

### Deploying the lab

Now that we have the topology and the configuration files in place, we can deploy the lab environment. To do so, we use the [`containerlab deploy`](../cmd/deploy.md) command:

```bash
containerlab deploy --topo ixp.clab.yaml
```

![deploy](https://gitlab.com/rdodin/pics/-/wikis/uploads/f45f3717c7f6a1a929523e05142ec352/2023-04-17_12-49-03_copy__1_.gif){.img-shadow}

Deployment time depends on the host machine specs but roughly may take 2 to 5 minutes. Of which 3 seconds is spent on actual deployment and the rest is spent on waiting for SR OS VM to boot up and accept SSH connections.

Upon successful deployment, containerlab presents the lab summary table that contains the information about the deployed nodes:

```
+---+----------------+--------------+-------------------------------+---------------+---------+----------------+----------------------+
| # |      Name      | Container ID |             Image             |     Kind      |  State  |  IPv4 Address  |     IPv6 Address     |
+---+----------------+--------------+-------------------------------+---------------+---------+----------------+----------------------+
| 1 | clab-ixp-peer1 | c9f5301899fb | sros:23.3.R1                  | vr-nokia_sros | running | 172.20.20.5/24 | 2001:172:20:20::5/64 |
| 2 | clab-ixp-peer2 | 83da54ce9f7b | quay.io/frrouting/frr:8.4.1   | linux         | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
| 3 | clab-ixp-rs1   | 701ee906f03f | quay.io/openbgpd/openbgpd:7.9 | linux         | running | 172.20.20.4/24 | 2001:172:20:20::4/64 |
| 4 | clab-ixp-rs2   | 7de1a2f30d52 | ghcr.io/srl-labs/bird:2.13    | linux         | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+----------------+--------------+-------------------------------+---------------+---------+----------------+----------------------+
```

This table contains vital information about the deployed nodes, such as the name, container ID, image, kind, state, IPv4 and IPv6 addresses. The names and IP addresses can be used to connect to the nodes via SSH if the node happens to run an SSH server.

### Inspecting the lab

At any point in time, containerlab users can refresh themselves on what is currently deployed in the lab environment by using the [`containerlab inspect`](../cmd/inspect.md) command:

```
$ containerlab inspect --all
+---+--------------+----------+----------------+--------------+-------------------------------+---------------+---------+----------------+----------------------+
| # |  Topo Path   | Lab Name |      Name      | Container ID |             Image             |     Kind      |  State  |  IPv4 Address  |     IPv6 Address     |
+---+--------------+----------+----------------+--------------+-------------------------------+---------------+---------+----------------+----------------------+
| 1 | ixp.clab.yml | ixp      | clab-ixp-peer1 | c9f5301899fb | sros:23.3.R1                  | vr-nokia_sros | running | 172.20.20.5/24 | 2001:172:20:20::5/64 |
| 2 |              |          | clab-ixp-peer2 | 83da54ce9f7b | quay.io/frrouting/frr:8.4.1   | linux         | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
| 3 |              |          | clab-ixp-rs1   | 701ee906f03f | quay.io/openbgpd/openbgpd:7.9 | linux         | running | 172.20.20.4/24 | 2001:172:20:20::4/64 |
| 4 |              |          | clab-ixp-rs2   | 7de1a2f30d52 | ghcr.io/srl-labs/bird:2.13    | linux         | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+--------------+----------+----------------+--------------+-------------------------------+---------------+---------+----------------+----------------------+
```

### Destroying the lab

One of containerlab principles is treating labs in a cattle-not-pets manner. This means that labs are ephemeral and can be destroyed at any point in time. To do so, we use the [`containerlab destroy`](../cmd/destroy.md) command:

```bash
containerlab destroy --cleanup -t ixp.clab.yml #(1)!
```

1. The `--cleanup` flag instructs containerlab to remove the [lab directory](../manual/conf-artifacts.md) that contains the lab configuration files.

## Accessing the lab nodes

To connect to the nodes of a running lab users can either use `ssh` client and the node name or IP address that containerlab assigned, or use the `docker exec` command. Typically the latter is used to connect to the nodes that do not run an SSH server, such as nodes `peer2`, `rs1` and `rs2`.

Nokia SR OS node runs an SSH server, therefore connecting to it is as simple as:

```bash title="connecting to Nokia SR OS node"
$ ssh admin@clab-ixp-peer1 #(1)!
admin@clab-ixp-peer1's password: 

SR OS Software
Copyright (c) Nokia 2023.  All Rights Reserved.

[/]
A:admin@peer1#
```

1. The `admin` user is the default user for the Nokia SR OS node. Password is `admin`.  
    `clab-ixp-peer1` is the name of the node as defined in the lab topology file with a lab prefix prepended to it.

For nodes without an SSH server running, we use the `docker exec` command to execute a process that runs in the context of the container and provides us with a shell:

``` title="connecting to FRRouting <code>vtysh</code>"
❯ docker exec -it clab-ixp-peer2 vtysh
% Can't open configuration file /etc/frr/vtysh.conf due to 'No such file or directory'.

Hello, this is FRRouting (version 8.4.1_git).
Copyright 1996-2005 Kunihiro Ishiguro, et al.

peer2#
```

The same connection method is used to connect to BIRD's `birdc` shell and OpenBGPd's `ash` shell:

* `docker exec -it clab-ixp-rs1 ash` - OpenBGPd doesn't have a shell, so we use ash to get into the container's shell from which we can execute `bgpctl` command line utility to drive OpenBGPd.
* `docker exec -it clab-ixp-rs2 birdc` - BIRD employs `birdc` shell which allows us to interact with BIRD via a command line interface.

## Basic service verification

The basic configuration that our peers and route servers were configured with assumed a plain unfiltered exchange of BGP routes. No IRR filtering, RPKI validation or any other MANRS-recommended security measures were enabled. This means that we can verify the basic connectivity between the peers and route servers by simply checking the BGP session status:

=== "Nokia SR OS"
    With the `show router bgp summary` command we can obtain the summarised BGP session status, where at the end of the output we can see that the BGP session with the two route servers was established successfully:
    ```bash title="Nokia SR OS node"
    [/]
    A:admin@peer1# show router bgp summary
    ===============================================================================
    BGP Router ID:10.0.0.1         AS:64501       Local AS:64501
    ===============================================================================
    BGP Admin State         : Up          BGP Oper State              : Up
    Total Peer Groups       : 1           Total Peers                 : 2
    Total VPN Peer Groups   : 0           Total VPN Peers             : 0
    Current Internal Groups : 1           Max Internal Groups         : 1
    Total BGP Paths         : 9           Total Path Memory           : 3200

    # -- snip --

    ===============================================================================
    BGP Summary
    ===============================================================================
    Legend : D - Dynamic Neighbor
    ===============================================================================
    Neighbor
    Description
                      AS PktRcvd InQ  Up/Down   State|Rcv/Act/Sent (Addr Family)
                          PktSent OutQ
    -------------------------------------------------------------------------------
    192.168.0.3
                    64503    2933    0 01d00h24m 1/1/1 (IPv4)
                             2934    0           
    192.168.0.4
                    64503    3346    0 01d00h24m 1/0/1 (IPv4)
                             2934    0           
    -------------------------------------------------------------------------------
    ```

    A single route has been sent to both peers (`10.0.0.1/32`) and a single route has been received from each peer. We can investigate which route has been received from the peers by using `show router bgp neighbor <neighbor addr> received-routes`:

    ```
    [/]
    A:admin@peer1# show router bgp neighbor "192.168.0.3" received-routes 
    ===============================================================================
    BGP Router ID:10.0.0.1         AS:64501       Local AS:64501      
    ===============================================================================
    Legend -
    Status codes  : u - used, s - suppressed, h - history, d - decayed, * - valid
                    l - leaked, x - stale, > - best, b - backup, p - purge
    Origin codes  : i - IGP, e - EGP, ? - incomplete

    ===============================================================================
    BGP IPv4 Routes
    ===============================================================================
    Flag  Network                                            LocalPref   MED
          Nexthop (Router)                                   Path-Id     IGP Cost
          As-Path                                                        Label
    -------------------------------------------------------------------------------
    u*>i  10.0.0.2/32                                        n/a         0
          192.168.0.2                                        None        0
          64502                                                          -
    -------------------------------------------------------------------------------
    Routes : 1
    ===============================================================================
    ```

    As we see, the FRR's route `10.0.0.2/32` has been received from the peer, as expected.
=== "FRRouting"
    On FRR side we can use `show ip bgp summary` to get the summarised status:
    ```title="FRRouting node"
    peer2# show ip bgp summary

    IPv4 Unicast Summary (VRF default):
    BGP router identifier 10.0.0.2, local AS number 64502 vrf-id 0
    BGP table version 2
    RIB entries 3, using 576 bytes of memory
    Peers 2, using 1434 KiB of memory

    Neighbor        V         AS   MsgRcvd   MsgSent   TblVer  InQ OutQ  Up/Down State/PfxRcd   PfxSnt Desc
    192.168.0.3     4      64503     29424     29426        0    0    0 1d00h31m            1        2 N/A
    192.168.0.4     4      64503     33613     29426        0    0    0 1d00h31m            1        2 N/A

    Total number of neighbors 2
    ```

## Use cases

Now that the foundation is in place, users are encouraged to explore the various use cases that are typical for an IXP setup. The following use cases may be covered in the updates to this lab in the future:

* Peer-side advanced filtering and BGP configuration
* IRR Filtering
* RPKI validation
* Looking glass integration
* ARouteServer-based provisioning
* IXP-manager introduction

## References

The following resources were used to create this lab:

* [AMS-IX Route Servers](https://www.ams-ix.net/ams/documentation/ams-ix-route-servers)
* [Implementation of RPKI and IRR filtering on the AMS-IX platform](https://www.ripe.net/support/training/ripe-ncc-educa/presentations/use-cases-stavros-konstantaras.pdf)
* [NL-IX Config Guide Peering](https://www.nl-ix.net/about/support/config-guide-peering/)
* [Nokia SR OS 23.3.1 BGP Guide](https://documentation.nokia.com/sr/23-3-1/books/unicast-routing-protocols/bgp-unicast-routing-protocols.html)
* [RFC 7948](https://datatracker.ietf.org/doc/html/rfc7948) & [RFC 7948](https://datatracker.ietf.org/doc/html/rfc7948)
* [Getting started with BIRD by Kintone](https://blog.kintone.io/entry/bird)
* [HowTo BIRD by dn42](https://dn42.eu/howto/Bird2)
* [IXP Lab by NSRC](https://nsrc.org/workshops/2021/riso-pern-apan51/networking/routing-security/en/labs/ixp.html)
* ARouteServer: [repo](https://github.com/pierky/arouteserver), [docs](https://arouteserver.readthedocs.io/en/latest/), [tutorial](https://www.youtube.com/watch?v=aiBeFs6xnYs)
* [bgpq4](https://github.com/bgp/bgpq4)
* [RPKI docs by NLNetLabs](https://rpki.readthedocs.io/en/latest/about/introduction.html)

[nokia-sros]: https://www.nokia.com/networks/technologies/service-router-operating-system/
[frr]: https://frrouting.org/
[openbgpd]: https://www.openbgpd.org/
[bird]: https://bird.network.cz/
[lab]: https://github.com/hellt/sros-frr-ixp-lab
[clab-install]: ../install.md
[bird-container]: https://github.com/srl-labs/bird-container/pkgs/container/bird
[obgpd-container]: https://quay.io/openbgpd/openbgpd:7.9
[rd-twitter]: https://twitter.com/ntdvps
[rd-linkedin]: https://www.linkedin.com/in/romandodin/
[frr-container]: https://quay.io/repository/frrouting/frr:8.4.1
[lab-file]: https://github.com/hellt/sros-frr-ixp-lab/blob/euro-ix/ixp.clab.yml
[sros-partial-cfg]: https://github.com/hellt/sros-frr-ixp-lab/blob/euro-ix/configs/sros.partial.cfg
[frr-conf-basic]: https://github.com/hellt/sros-frr-ixp-lab/blob/euro-ix/configs/frr.conf
[frr-daemons-basic]: https://github.com/hellt/sros-frr-ixp-lab/blob/euro-ix/configs/frr-daemons.cfg
[obgpd-conf-basic]: https://github.com/hellt/sros-frr-ixp-lab/blob/euro-ix/configs/openbgpd.conf
[bird-conf-basic]: https://github.com/hellt/sros-frr-ixp-lab/blob/euro-ix/configs/bird.conf

[^1]: Container image is based on the pierky/bird container image, but with iproute2 package installed.
