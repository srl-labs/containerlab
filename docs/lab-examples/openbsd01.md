|                               |                                                                        |
| ----------------------------- | ---------------------------------------------------------------------- |
| **Description**               | A OpenBSD connected to two Alpine Linux Hosts                          |
| **Components**                | [OpenBSD][openbsd], [Multitool Alpine Linux][client]                   |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 512 MB |
| **Topology file**             | [openbsd01.yml][topofile]                                              |
| **Name**                      | openbsd01                                                              |
| **Version information**[^2]   | `containerlab:0.49.0`, `openbsd-7.3-2023-04-22.qcow2`, `docker:24.0.6` |

## Description

This lab consists of one OpenBSD router connected to two Alpine Linux nodes.

```
client1<---->OpenBSD<---->client2
```

## Configuration

The OpenBSD node takes about 1 minute to complete its start up. Check using "docker container ls" until the OpenBSD container shows up as "healthy".

```
# docker container ls
CONTAINER ID   IMAGE                                  COMMAND                  CREATED              STATUS                        PORTS                                       NAMES
6fee243af470   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   About a minute ago   Up About a minute             80/tcp, 443/tcp, 1180/tcp, 11443/tcp        clab-openbsd01-client1
46800cd24290   vrnetlab/vr-openbsd:7.3                "/launch.py --userna…"   About a minute ago   Up About a minute (healthy)   22/tcp, 5000/tcp, 10000-10099/tcp           clab-openbsd01-obsd1
ce53649d8741   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   About a minute ago   Up About a minute             80/tcp, 443/tcp, 1180/tcp, 11443/tcp        clab-openbsd01-client2
```

### obsd01

Log into the OpenBSD node using SSH with `ssh admin@clab-openbsd01-obsd1` and add the following configuration. Password is `admin`.

```
sudo sysctl net.inet.ip.forwarding=1
sudo ifconfig vio1 192.168.1.1/30
sudo ifconfig vio2 192.168.2.1/30
```

### client1

The two clients should be configured with the correct IP addresses and a route to the other client via the OpenBSD node.
First attach to the container process `docker exec -it clab-openbsd01-client1 ash`

```
docker exec -it clab-openbsd01-client1 ash

# ip -br a show dev eth1
eth1@if3428      UP             192.168.1.2/30 fe80::a8c1:abff:fefd:9fcf/64

# ip r
default via 172.20.20.1 dev eth0
172.20.20.0/24 dev eth0 proto kernel scope link src 172.20.20.3
192.168.1.0/30 dev eth1 proto kernel scope link src 192.168.1.2
192.168.2.0/30 via 192.168.1.1 dev eth1
```

## Verification

Traceroute from client1 to client2 to verify the data-plane via the OpenBSD node.

### client1

```
# traceroute 192.168.2.2
traceroute to 192.168.2.2 (192.168.2.2), 30 hops max, 46 byte packets
 1  192.168.1.1 (192.168.1.1)  0.874 ms  0.484 ms  0.151 ms
 2  192.168.2.2 (192.168.2.2)  0.240 ms  0.182 ms  0.148 ms
```
  
[openbsd]: https://www.openbsd.org/
[client]: https://github.com/wbitt/Network-MultiTool
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/openbsd01/openbsd01.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
