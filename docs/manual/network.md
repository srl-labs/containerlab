<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

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
    nokia_srlinux:
      type: ixrd3
      image: ghcr.io/nokia/srlinux
  nodes:
    srl1:
      kind: nokia_srlinux
    srl2:
      kind: nokia_srlinux

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
    We often refer to `clab` docker network simply as _management network_ since it's the network to which management interfaces of the containerized NOS'es are connected.

The addressing information that containerlab will use on this network:

* IPv4: subnet 172.20.20.0/24, gateway 172.20.20.1
* IPv6: subnet 3fff:172:20:20::/64, gateway 3fff:172:20:20::1

This management network will be configured with MTU value matching the value of a `docker0` host interface to match docker configuration on the system. This option is [configurable](#mtu).

With these defaults in place, the two containers from this lab will get connected to that management network and will be able to communicate using the IP addresses allocated by docker daemon. The addresses that docker carves out for each container are presented to a user once the lab deployment finishes or can be queried any time after:

```bash
# addressing information is available once the lab deployment completes
❯ containerlab deploy -t srl02.clab.yml
# deployment log omitted for brevity
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| # |      Name       | Container ID |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| 1 | clab-srl02-srl1 | ca24bf3d23f7 | srlinux | srl  |       | running | 172.20.20.3/24 | 3fff:172:20:20::3/80 |
| 2 | clab-srl02-srl2 | ee585eac9e65 | srlinux | srl  |       | running | 172.20.20.2/24 | 3fff:172:20:20::2/80 |
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+

# addresses can also be fetched afterwards with `inspect` command
❯ containerlab inspect -a
+---+----------+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| # | Lab Name |      Name       | Container ID |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+----------+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| 1 | srl02    | clab-srl02-srl1 | ca24bf3d23f7 | srlinux | srl  |       | running | 172.20.20.3/24 | 3fff:172:20:20::3/80 |
| 2 | srl02    | clab-srl02-srl2 | ee585eac9e65 | srlinux | srl  |       | running | 172.20.20.2/24 | 3fff:172:20:20::2/80 |
+---+----------+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
```

The output above shows that srl1 container has been assigned `172.20.20.3/24 / 3fff:172:20:20::3/80` IPv4/6 address. We can ensure this by querying the srl1 management interfaces address info:

```bash
❯ docker exec clab-srl02-srl1 ip address show dummy-mgmt0
6: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    link/ether 2a:66:2b:09:2e:4d brd ff:ff:ff:ff:ff:ff
    inet 172.20.20.3/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever
    inet6 3fff:172:20:20::3/80 scope global
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
  ipv4-subnet: 172.100.100.0/24       # ipv4 range
  ipv6-subnet: 3fff:172:100:100::/80  # ipv6 range (optional)

topology:
# the rest of the file is omitted for brevity
```

With these settings in place, the container will get their IP addresses from the specified ranges accordingly.

#### user-defined addresses

By default, container runtime will assign the management IP addresses for the containers. But sometimes, it's helpful to have user-defined addressing in the management network.

For such cases, users can define the desired IPv4/6 addresses on a per-node basis:

```yaml
mgmt:
  network: fixedips
  ipv4-subnet: 172.100.100.0/24
  ipv6-subnet: 3fff:172:100:100::/80

topology:
  nodes:
    n1:
      kind: nokia_srlinux
      mgmt-ipv4: 172.100.100.11       # set ipv4 address on management network
      mgmt-ipv6: 3fff:172:100:100::11 # set ipv6 address on management network
```

Users can specify either IPv4 or IPv6 or both addresses. If one of the addresses is omitted, it will be assigned by container runtime in an arbitrary fashion.

!!!note
    1. If user-defined IP addresses are needed, they must be provided for all containers attached to a given network to avoid address collision.
    2. IPv4/6 addresses set on a node level must be from the management network range.
    3. IPv6 addresses are truncated by Docker[^1], therefore do not use bytes 5 through 8 of the IPv6 network range.

#### auto-assigned addresses

The default network addresses chosen by containerlab - 172.20.20.0/24 and 3fff:172:20:20::/64 - may clash with the existing addressing scheme on the lab host. With the [user-defined addresses](#user-defined-addresses) discussed above, users can avoid such conflicts, but this requires manual changes to the lab topology file and may not be convenient.

To address this issue, containerlab provides a way to automatically assign the management network v4/v6 addresses. This is achieved by setting the `ipv4-subnet` and/or `ipv6-subnet` to `auto`:

```yaml
mgmt:
  ipv4-subnet: auto
  ipv6-subnet: auto
```

With this setting in place, containerlab will rely on the container runtime to assign the management network addresses that is not conflicting with the existing addressing scheme on the lab host.

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

If the existing bridge has already been addressed with IPv4/6 address, containerlab will respect this address and use it in the IPAM configuration blob of the docker network.

If there is no existing IPv4/6 address defined for the custom bridge, docker will assign the first interface from the subnet associated with the bridge.

It is possible to set the desired gateway IP (that is the IP assigned to the bridge) with the `ipv4-gw/ipv6-gw` setting under `mgmt` container:

```yaml
mgmt:
  network: custom-net
  bridge: mybridge
  ipv4-subnet: 10.20.30.0/24 # ip range for the docker network
  ipv4-gw: 10.20.30.100 # set custom gateway ip
```

#### IP range

By specifying `ipv4-range/ipv6-range` under the management network, users limit the network range from which IP addresses are allocated for a management subnet.

```yaml
mgmt:
  network: custom-net
  ipv4-subnet: 10.20.30.0/24 #(2)!
  ipv4-range: 10.20.30.128/25 #(1)!
```

1. Container runtime will assign IP addresses from the `10.20.30.128/25` subnet, and `10.20.30.0/25` will not be considered.
2. The subnet must be specified for IP ranges to work. Also note that if the container network already exists and uses a different range, then the IP range setting won't have effect.

With this approach, users can prevent IP address overlap with nodes deployed on the same management network by other orchestration systems.

#### external access

Containerlab will attempt to enable external management access to the nodes by default. This means that external systems/hosts will be able to communicate with the nodes of your topology without requiring any manual iptables/nftables rules to be installed.

To allow external communications containerlab installs a rule in the `DOCKER-USER` chain for v4 and v6, allowing all packets targeting containerlab's management network. The rule looks like follows:

```shell
sudo iptables -vnL DOCKER-USER
```

<div class="embed-result">
```{.no-copy .no-select}
Chain DOCKER-USER (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 ACCEPT     0    --  br-1351328e1855 *       0.0.0.0/0            0.0.0.0/0            /* set by containerlab */
    0     0 ACCEPT     0    --  *      br-1351328e1855  0.0.0.0/0            0.0.0.0/0            /* set by containerlab */
    0     0 RETURN     0    --  *      *       0.0.0.0/0            0.0.0.0/0
```
</div>

1. The `br-1351328e1855` bridge interface is the interface that backs up the containerlab's management network (`clab` docker network).

The rule will be removed together with the management network.

///tip | RHEL 9 users
By default RHEL 9 (and it's derivatives) will use `firewalld` as the [default firewall](https://access.redhat.com/solutions/7046655), containerlab's `iptables` and `nftables` rules will not work in this case and you will not have external access to your labs.

To fix this you must disable `firewalld` and enable the `nftables` service.  

**Take caution when disabling firewalls, you may be exposing things you shouldn't**

```
systemctl disable firewalld
systemctl stop firewalld
systemctl mask firewalld

systemctl enable --now nftables
```

///

Should you not want to enable external access to your nodes you can set `external-access` property to `false` under the management section of a topology:

```yaml
name: no-ext-access
mgmt:
  external-access: false #(1)!
topology:
# your regular topology definition
```

1. When set to `false`, containerlab will not touch iptables rules. On most docker installations this will result in restricted external access.

///details | Errors and warnings
External access feature requires nftables kernel API to be present. Kernels newer than v4 typically have this API enabled by default. To understand which API is in use one can issue the following command:

```bash
iptables -V
iptables v1.8.5 (nf_tables)
```

If the outputs contains `nf_tables` you are all set. If it contains `legacy` or doesn't say anything about `nf_tables` then nf_tables API is not available and containerlab will not be able to setup external access. You will have to enable it manually (or better yet - upgrade the kernel).  
Older distros, like Centos 7, are known to use the legacy iptables backend and therefore will emit a warning when containerlab will attempt to launch a lab. The warning will not prevent the lab from starting and running, but you will need to setup iptables rules manually if you want your nodes to be accessible from the outside of your containerlab host.

Containerlab will throw an error "missing DOCKER-USER iptables chain" when this chain is not found. This error is typically caused by two factors

1. Old docker version installed. Typically seen on Centos systems. Minimum required docker version is 17.06.
2. Docker is installed incorrectly. It is recommended to follow the [official installation procedures](https://docs.docker.com/engine/install/) by selecting "Installation per distro" menu option.

When docker is correctly installed, additional iptables chains will become available and the error will not appear.
///

### bridge network driver options

By default, containerlab will create the management bridge with default driver options[^2], however, for special networking setups required in some cases, this can be overridden in the `driver-opts` section of the `mgmt` block.

This section is a key-value pair for each overridden driver option:

```yaml
mgmt:
  network: custom-net
  bridge: mybridge
  driver-opts:
    com.docker.network.bridge.gateway_mode_ipv4: routed
    com.docker.network.bridge.gateway_mode_ipv6: routed
```

All driver options can be overridden, even those set by containerlab.

### connection details

When containerlab needs to create the management network, it asks the docker daemon to do this. Docker will fulfill the request and will create a network with the underlying linux bridge interface backing it. The bridge interface name is generated by the docker daemon, but it is easy to find it:

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

# now the name is known and it's easy to show bridge state
❯ brctl show br-d2169a14e334
bridge name         bridge id      STP enabled   interfaces
br-d2169a14e334  8000.0242fe382b74 no        vetha57b950
                                 vethe9da10a
```

As explained in the beginning of this article, containers will connect to this docker network. This connection is carried out by the `veth` devices created and attached with one end to bridge interface in the lab host and the other end in the container namespace. This is illustrated by the bridge output above and the diagram at the beginning the of the article.

## Point-to-point links

Management network is used to provide management access to the NOS containers, it does not carry control or dataplane traffic. In containerlab we create additional point-to-point links between the containers to provide the datapath between the lab nodes.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:11,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

The above diagram shows how links are created in the topology definition file. In this example, the datapath consists of the two virtual point-to-point wires between SR Linux and cEOS containers. These links are created on-demand by containerlab itself.

The p2p links are typically provided by the `veth` device pairs where each end of the `veth` pair is attached to a respective container.

### Link MTU

The MTU on the veth links is set by default to 9500B, so a regular jumbo frame shouldn't traverse the links without problems. If you need to change the MTU, you can do so by setting the `mtu` property in the link definition:

```yaml
topology:
  links:
    - endpoints: ["router2:eth2", "router3:eth1"]
      mtu: 1500
```

### Host links

It is also possible to interconnect container' data interface not with other container or add it to a [bridge](kinds/bridge.md), but to attach it to a host's root namespace. This is, for example, needed to create a L2 connectivity between containerlab nodes running on different VMs (aka multi-node labs).

This "host-connectivity" is achieved by using a reserved node name - `host` - referenced in the endpoints section. Consider the following example where an SR Linux container has its only data interface connected to a hosts root namespace via veth interface:

```yaml
name: host

topology:
  nodes:
    srl:
      kind: nokia_srlinux
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
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
  links:
    - endpoints:
        - "n1:e1-1"
        - "mgmt-net:n1-e1-1"

```

In the above example the node `n1` connects with its `e1-1` interface to the management network. This is done by specifying the endpoint with a reserved name `mgmt-net` and defining the name of the interface that should be used in that bridge (`n1-e1-1`).

By specifying `mgmt-net` name of the node in the endpoint definition we tell containerlab to find out which bridge is used by the management network of our lab and use this bridge as the attachment point for our veth pair.

This is best illustrated with the following diagram:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:14,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

### MACVLAN links

In addition to the `veth` links, containerlab supports `macvlan` links. This type of links is useful when users want to connect containers to the host interface/network directly. This is achieved by defining a link endpoints which has one end defined with a special `macvlan:<host-iface-name>` signature.

Consider the following example where we connect a Linux container `l1` to the hosts `enp0s3` interface:

```yaml
name: macvlan

topology:
  nodes:
    l1:
      kind: linux
      image: alpine:3

  links:
    - endpoints: ["l1:eth1", "macvlan:enp0s3"]
```

This topology will result in l1 node having its `eth1` interface connected to the `enp0s3` interface of the host as per the diagram below:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph='{"page":16,"zoom":1.5,"highlight":"#0000ff","nav":true,"check-visible-state":true,"resize":true,"url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio"}'></div>

Containerlab will create a macvlan interface in the bridge mode, attach it to the parent `enp0s3` interface and then move it to the container's net namespace and name it `eth1`, as instructed by the endpoint definition in the topology.

Users then can configure the `eth1` interface inside the container as they would do with any other interface. As per the diagram above, we configure `eth1` interface with ipv4 address from the host's `enp0s3` interface subnet:

```bash title="entering the container's shell"
docker exec -it clab-macvlan-l1 ash
```

```bash
# adding v4 address to the eth1 interface
ip address add 10.0.0.111/24 dev eth1
```

Once v4 address is assigned to the macvlan inteface, we can test the connectivity by pinging default gateway of the host:

```bash
❯ ping 10.0.0.1
PING 10.0.0.1 (10.0.0.1): 56 data bytes
64 bytes from 10.0.0.1: seq=0 ttl=64 time=0.545 ms
64 bytes from 10.0.0.1: seq=1 ttl=64 time=0.243 ms
```

When capturing packets from the hosts's `enp0s3` interface we can see that the ping packets are coming through it using the mac address assigned to the macvlan inteface:

```bash
❯ tcpdump -nnei enp0s3 icmp
tcpdump: verbose output suppressed, use -v[v]... for full protocol decode
listening on enp0s3, link-type EN10MB (Ethernet), snapshot length 262144 bytes
22:35:02.430901 aa:c1:ab:72:b3:fe > fa:16:3e:af:03:05, ethertype IPv4 (0x0800), length 98: 10.0.0.111 > 10.0.0.1: ICMP echo request, id 24, seq 4, length 64
22:35:02.431017 fa:16:3e:af:03:05 > aa:c1:ab:72:b3:fe, ethertype IPv4 (0x0800), length 98: 10.0.0.1 > 10.0.0.111: ICMP echo reply, id 24, seq 4, length 64
```

## Manual control over the management network

By default containerlab creates a docker network named `clab` and attaches all the nodes to this network. This network is used as a management network for the nodes and is managed by the container runtime such as docker or podman.

Container runtime is responsible for creating the `eth0` interface inside the container and attaching it to the `clab` network. This interface is used by the container to communicate with the management network. For that reason the links users create in the topology's `links` section typically do not include the `eth0` interface.

However, there might be cases when users want to take control over `eth0` interface management. For example, they might want to connect `eth0` to a different network or even another container's interface. To achieve that, users can instruct container runtime to not manage the `eth0` interface and leave it to the user, using [`network-mode: none`](nodes.md#network-mode) setting.

Consider the following example, where node1's management interface `eth0` is provided in the `links` section and connects to node2's `eth1` interface:

```yaml
name: e0

topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3
      network-mode: none
    node2:
      kind: linux
      image: alpine:3
  links:
    - endpoints: ["node1:eth0", "node2:eth1"]
```

## DNS

When containerlab finishes the nodes deployment, it also creates static DNS entries inside the `/etc/hosts` file so that users can access the nodes using their DNS names.

The DNS entries are created for each node's IPv4/6 address, and follow the pattern - `clab-$labName-$nodeName`.

For a lab named `demo` with two nodes named `l1` and `l2` containerlab will create the following section inside the `/etc/hosts` file.

```
###### CLAB-demo-START ######
172.20.20.2     clab-demo-l1
172.20.20.3     clab-demo-l2
3fff:172:20:20::2       clab-demo-l1
3fff:172:20:20::3       clab-demo-l2
###### CLAB-demo-END ######
```

[^1]: See <https://github.com/srl-labs/containerlab/issues/1302#issuecomment-1533796941> for details and links to the original discussion.
[^2]: The only exception to this is setting the gateway mode to `nat-unprotected` for Docker version 28 and above, see <https://github.com/srl-labs/containerlab/issues/2638> for the original discussion.
