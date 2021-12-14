<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>
One of the most important tasks in the process of building container based labs is to create a virtual wiring between the containers and the host. That is one of the problems that containerlab was designed to solve.

In this document we will discuss the networking concepts that containerlab employs to provide the following connectivity scenarios:

1. Make containers available from the lab host
2. Interconnect containers to create network topologies of users choice

## Management network
As governed by the well-established container [networking principles](https://docs.docker.com/network/) containers are able to get network connectivity using various drivers/methods. The most common networking driver that is enabled by default for docker-managed containers is the [bridge driver](https://docs.docker.com/network/bridge/).

The bridge driver connects containers to a linux bridge interface named `docker0` on most linux operating systems. The containers are then able to communicate with each other and the host via this virtual switch (bridge interface).

In containerlab we follow a similar approach: containers launched by containerlab will be attached with their interface to a containerlab-managed docker network. It's best to be explained by an example which we will base on a [two nodes](../lab-examples/two-srls.md) lab from our catalog:

```yaml
name: srl02

topology:
  kinds:
    srl:
      type: ixr6
      image: ghcr.io/nokia/srlinux
  nodes:
    srl1:
      kind: srl
    srl2:
      kind: srl

  links:
    - endpoints: ["srl1:e1-1", "srl2:e1-1"]
```

As seen from the topology definition file, the lab consists of the two SR Linux nodes which are interconnected via a single point-to-point link.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:3,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

The diagram above shows that these two nodes are not only interconnected between themselves, but also connected to a bridge interface on the lab host. This is driven by the containerlab default management network settings.

### default settings
When no information about the management network is provided within the topo definition file, containerlab will do the following

1. create, if not already created, a docker network named `clab`
2. configure the IPv4/6 addressing pertaining to this docker network

!!!info
    We often refer to `clab` docker network simply as _management network_ since its the network to which management interfaces of the containerized NOS'es are connected.

The addressing information that containerlab will use on this network:

* IPv4: subnet 172.20.20.0/24, gateway 172.20.20.1
* IPv6: subnet 2001:172:20:20::/64, gateway 2001:172:20:20::1

This management network will be configured with MTU value matching the value of a `docker0` host interface to match docker configuration on the system. This option is [configurable](#mtu).

With these defaults in place, the two containers from this lab will get connected to that management network and will be able to communicate using the IP addresses allocated by docker daemon. The addresses that docker carves out for each container are presented to a user once the lab deployment finishes or can be queried any time after:

```bash
# addressing information is available once the lab deployment completes
❯ containerlab deploy -t srl02.clab.yml
# deployment log omitted for brevity
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| # |      Name       | Container ID |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| 1 | clab-srl02-srl1 | ca24bf3d23f7 | srlinux | srl  |       | running | 172.20.20.3/24 | 2001:172:20:20::3/80 |
| 2 | clab-srl02-srl2 | ee585eac9e65 | srlinux | srl  |       | running | 172.20.20.2/24 | 2001:172:20:20::2/80 |
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+

# addresses can also be fetched afterwards with `inspect` command
❯ containerlab inspect -a
+---+----------+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| # | Lab Name |      Name       | Container ID |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+----------+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| 1 | srl02    | clab-srl02-srl1 | ca24bf3d23f7 | srlinux | srl  |       | running | 172.20.20.3/24 | 2001:172:20:20::3/80 |
| 2 | srl02    | clab-srl02-srl2 | ee585eac9e65 | srlinux | srl  |       | running | 172.20.20.2/24 | 2001:172:20:20::2/80 |
+---+----------+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
```

The output above shows that srl1 container has been assigned `172.20.20.3/24 / 2001:172:20:20::3/80` IPv4/6 address. We can ensure this by querying the srl1 management interfaces address info:

```bash
❯ docker exec clab-srl02-srl1 ip address show dummy-mgmt0
6: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    link/ether 2a:66:2b:09:2e:4d brd ff:ff:ff:ff:ff:ff
    inet 172.20.20.3/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever
    inet6 2001:172:20:20::3/80 scope global
       valid_lft forever preferred_lft forever
```

Now it's possible to reach the assigned IP address from the lab host as well as from other containers connected to this management network.
```bash
# ping srl1 management interface from srl2
❯ docker exec -it clab-srl02-srl2 sr_cli "ping 172.20.20.3 network-instance mgmt"
x -> $176x48
Using network instance mgmt
PING 172.20.20.3 (172.20.20.3) 56(84) bytes of data.
64 bytes from 172.20.20.3: icmp_seq=1 ttl=64 time=2.43 ms
```

!!!note
    If you run multiple labs without changing the default management settings, the containers of those labs will end up connecting to the same management network with their management interface.

### host mode networking
In addition to the bridge-based management network containerlab supports launching nodes in [host networking mode](https://docs.docker.com/network/host/). In this mode containers are attached to the host network namespace. Host mode is enabled with [network-mode](nodes.md#network-mode) node setting.

### configuring management network
Most of the time there is no need to change the defaults for management network configuration, but sometimes it is needed. For example, it might be that the default network ranges are overlapping with the existing addressing scheme on the lab host, or it might be desirable to have predefined management IP addresses.

For such cases, the users need to add the `mgmt` container at the top level of their topology definition file:

```yaml
name: srl02

mgmt:
  network: custom_mgmt                # management network name
  ipv4_subnet: 172.100.100.0/24       # ipv4 range
  ipv6_subnet: 2001:172:100:100::/80  # ipv6 range (optional)

topology:
# the rest of the file is omitted for brevity
```

With these settings in place, container will get their IP addresses from the specified ranges accordingly.

#### user-defined addresses
By default, container runtime will assign the management IP addresses for the containers. But sometimes, it's helpful to have user-defined addressing in the management network.

For such cases, users can define the desired IPv4/6 addresses on a per-node basis:

```yaml
mgmt:
  network: fixedips
  ipv4_subnet: 172.100.100.0/24
  ipv6_subnet: 2001:172:100:100::/80

topology:
  nodes:
    n1:
      kind: srl
      mgmt_ipv4: 172.100.100.11       # set ipv4 address on management network
      mgmt_ipv6: 2001:172:100:100::11 # set ipv6 address on management network
```

Users can specify either IPv4 or IPv6 or both addresses. If one of the addresses is omitted, it will be assigned by container runtime in an arbitrary fashion.

!!!note
    1. If user-defined IP addresses are needed, they must be provided for all containers attached to a given network to avoid address collision.
    2. IPv4/6 addresses set on a node level must be from the management network range.

#### MTU
The MTU of the management network defaults to an MTU value of `docker0` interface, but it can be set to a user defined value:

```yaml
mgmt:
  network: clab_mgmt
  mtu: 2100 # set mtu of the management network to 2100
```

This will result in every interface connected to that network to inherit this MTU value.

#### network name
The default container network name is `clab`. To customize this name, users should specify a new value within the `network` element:

```yaml
mgmt:
  network: myNetworkName
```

#### default docker network
To make clab nodes start in the default docker network `bridge`, which uses the `docker0` bridge interface, users need to mention this explicitly in the configuration:

```yaml
mgmt:
  network: bridge
```

Since `bridge` network is created by default by docker, using its name in the configuration will make nodes to connect to this network.

#### bridge name
By default, containerlab will create a linux bridge backing the management docker network with the following name `br-<network-id>`. The network-id part is coming from the docker network ID that docker manages.

We allow our users to change the bridge name that the management network will use. This can be used to connect containers to an already existing bridge with other workloads connected:

```yaml
mgmt:
  # a bridge with a name mybridge will be created or reused
  # as a backing bridge for the management network
  bridge: mybridge
```

### connection details
When containerlab needs to create the management network it asks the docker daemon to do this. Docker will fullfil the request and will create a network with the underlying linux bridge interface backing it. The bridge interface name is generated by the docker daemon, but it is easy to find it:

```bash
# list existing docker networks
# notice the presence of the `clab` network with a `bridge` driver
❯ docker network ls
NETWORK ID          NAME                DRIVER              SCOPE
5d60b6ec8420        bridge              bridge              local
d2169a14e334        clab                bridge              local
58ec5037122a        host                host                local
4c1491a09a1a        none                null                local

# the underlying linux bridge interface name follows the `br-<first_12_chars_of_docker_network_id> pattern
# to find the network ID use:
❯ docker network inspect clab -f {{.ID}} | head -c 12
d2169a14e334

# now the name is known and its easy to show bridge state
❯ brctl show br-d2169a14e334
bridge name	        bridge id		    STP enabled	  interfaces
br-d2169a14e334		8000.0242fe382b74	no		      vetha57b950
							                          vethe9da10a
```

As explained in the beginning of this article, containers will connect to this docker network. This connection is carried out by the `veth` devices created and attached with one end to bridge interface in the lab host and the other end in the container namespace. This is illustrated by the bridge output above and the diagram at the beginning the of the article.

## Point-to-point links
Management network is used to provide management access to the NOS containers, it does not carry control or dataplane traffic. In containerlab we create additional point-to-point links between the containers to provide the datapath between the lab nodes.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:11,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

The above diagram shows how links are created in the topology definition file. In this example, the datapath consists of the two virtual point-to-point wires between SR Linux and cEOS containers. These links are created on-demand by containerlab itself.

The p2p links are provided by the `veth` device pairs where each end of the `veth` pair is attached to a respective container. The MTU on these veth links is set to 9500, so a regular 9212 MTU on the network links shouldn't be a problem.

### host links
It is also possible to interconnect container' data interface not with other container or add it to a [bridge](kinds/bridge.md), but to attach it to a host's root namespace. This is, for example, needed to create a L2 connectivity between containerlab nodes running on different VMs (aka multi-node labs).

This "host-connectivity" is achieved by using a reserved node name - `host` - referenced in the endpoints section. Consider the following example where an SR Linux container has its only data interface connected to a hosts root namespace via veth interface:

```yaml
name: host

topology:
  nodes:
    srl:
      kind: srl
      image: ghcr.io/nokia/srlinux
      startup-config: test-srl-config.json
  links:
    - endpoints: ["srl:e1-1", "host:srl_e1-1"]
```

With this topology definition, we will have a veth interface with its one end in the container' namespace and its other end in the host namespace. The host will have the interface named `srl_e1-1` once the lab deployed:

```bash
ip link
# SNIP
433: srl_e1-1@if434: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default 
    link/ether b2:80:e9:60:c7:9d brd ff:ff:ff:ff:ff:ff link-netns clab-srl01-srl
```

### Additional connections to management network
By default every lab node will be connected to the docker network named `clab` which acts as a management network for the nodes.

In addition to that mandatory connection, users can attach additional interfaces to this management network. This might be needed, for example, when data interface of a node needs to talk to the nodes on the management network.

For such connections a special form of endpoint definition was created - `mgmt-net:$iface-name`.

```yaml
name: mgmt
topology:
  nodes:
    n1:
      kind: srl
      image: ghcr.io/nokia/srlinux
  links:
    - endpoints:
        - "n1:e1-1"
        - "mgmt-net:n1-e1-1"

```

In the above example the node `n1` connects with its `e1-1` interface to the management network. This is done by specifying the endpoint with a reserved name `mgmt-net` and defining the name of the interface that should be used in that bridge (`nq-e1-1`).

By specifying `mgmt-net` name of the node in the endpoint definition we tell containerlab to find out which bridge is used by the management network of our lab and use this bridge as the attachment point for our veth pair.

This is best illustrated with the following diagram:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:14,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

## DNS
When containerlab finishes the nodes deployment, it also creates static DNS entries inside the `/etc/hosts` file so that users can access the nodes using their DNS names.

The DNS entries are created for each node's IPv4/6 address, and follow the pattern - `clab-$labName-$nodeName`.

For a lab named `demo` with two nodes named `l1` and `l2` containerlab will create the following section inside the `/etc/hosts` file.

```
###### CLAB-demo-START ######
172.20.20.2     clab-demo-l1
172.20.20.3     clab-demo-l2
2001:172:20:20::2       clab-demo-l1
2001:172:20:20::3       clab-demo-l2
###### CLAB-demo-END ######
```
