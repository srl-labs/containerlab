|                               |                                                                                    |
| ----------------------------- | ---------------------------------------------------------------------------------- |
| **Description**               | A Juniper vSRX connected to two Alpine Linux Hosts                                 |
| **Components**                | [Juniper vSRX][vsrx], [Multitool Alpine Linux][client]                             |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 4 GB               |
| **Topology file**             | [vsrx01.yml][topofile]                                                             |
| **Name**                      | vsrx01                                                                             |
| **Version information**[^2]   | `containerlab:0.47.2`, `junos-vsrx3-x86-64-23.2R1.13.qcow2`, `docker:24.0.6`       |

## Description

This lab consists of one Juniper vSRX router connected to two Alpine Linux nodes.

```
client1<---->vSRX<---->client2
```

### Configuration

The vSRX takes about 5 minutes to complete its start up. Check using "docker container ls" until the vSRX shows up as "healthy"

```
# docker container ls
CONTAINER ID   IMAGE                                  COMMAND                  CREATED          STATUS                    PORTS                                        NAMES
85e3251a27c1   vrnetlab/vr-vsrx:23.2R1.13             "/launch.py --userna…"   10 minutes ago   Up 10 minutes (healthy)   22/tcp, 830/tcp, 5000/tcp, 10000-10099/tcp   clab-vsrx1-srx1
f06a4997ac1b   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   10 minutes ago   Up 10 minutes             80/tcp, 443/tcp, 1180/tcp, 11443/tcp         clab-vsrx1-client1
c77b68244805   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   10 minutes ago   Up 10 minutes             80/tcp, 443/tcp, 1180/tcp, 11443/tcp         clab-vsrx1-client2
```

#### vsrx1

Log into the vSRX using SSH with `ssh admin@clab-vsrx1-srx1` and add the configuration from srx01.cfg. Password is `admin@123`.

```
admin>configure
set interfaces ge-0/0/0 unit 0 family inet address 192.168.1.1/30
set interfaces ge-0/0/1 unit 0 family inet address 192.168.2.1/30
set security zones security-zone trust interfaces ge-0/0/0 host-inbound-traffic system-services all
set security zones security-zone trust interfaces ge-0/0/1 host-inbound-traffic system-services all
set system services web-management https system-generated-certificate
set security forwarding-options family mpls mode packet-based
# commit 
```

#### client1

The two clients should be configured with the correct IP addresses and a route to the other client via the vSRX.
First attach to the container process `docker exec -it clab-vsrx1-client1 ash`

```
docker exec -it clab-vsrx1-client1 ash

# ip a show dev eth1
131: eth1@if132: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 9500 qdisc noqueue state UP group default
   link/ether aa:c1:ab:ac:1b:19 brd ff:ff:ff:ff:ff:ff link-netnsid 1
   inet 192.168.1.2/30 scope global eth1
      valid_lft forever preferred_lft forever
   inet6 fe80::a8c1:abff:feac:1b19/64 scope link
      valid_lft forever preferred_lft forever

# ip route
default via 172.20.20.1 dev eth0
172.20.20.0/24 dev eth0 proto kernel scope link src 172.20.20.4
192.168.1.0/30 dev eth1 proto kernel scope link src 192.168.1.2
192.168.2.0/30 via 192.168.1.1 dev eth1
```

### Verification

Traceroute from client1 to client2 to verify the dataplane via the vSRX.

#### client1

```
# traceroute 192.168.2.2
traceroute to 192.168.2.2 (192.168.2.2), 30 hops max, 46 byte packets
1  192.168.1.1 (192.168.1.1)  0.397 ms  0.347 ms  0.290 ms
2  192.168.2.2 (192.168.2.2)  0.263 ms  0.374 ms  0.762 ms
```

#### vSRX Web Gui

To access the vSRX web interface point a browsers at the vSRX management IP address (fxp0) and use https. Login is `admin/admin@123`.
  
[vsrx]: https://www.juniper.net/us/en/products/security/srx-series/vsrx-virtual-firewall-datasheet.html
[client]: https://github.com/wbitt/Network-MultiTool
[topofile]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/vsrx01/vsrx01.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
