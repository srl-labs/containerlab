|                               |                                                                        |
| ----------------------------- | ---------------------------------------------------------------------- |
| **Description**               | A FreeBSD connected to two Alpine Linux Hosts                          |
| **Components**                | [FreeBSD][freebsd], [Multitool Alpine Linux][client]                   |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 512 MB |
| **Topology file**             | [freebsd01.clab.yml][topofile]                                              |
| **Name**                      | freebsd01                                                              |
| **Version information**[^2]   | `freebsd-13.2-zfs-2023-04-21.qcow2`, `docker:24.0.6` |

## Description

This lab consists of one FreeBSD router connected to two Alpine Linux nodes.

```
client1<---->FreeBSD<---->client2
```

## Configuration

The FreeBSD node takes about 1 minute to complete its start up. Check using "docker container ls" until the FreeBSD container shows up as "healthy".

```
# docker container ls
CONTAINER ID   IMAGE                                  COMMAND                  CREATED              STATUS                        PORTS                                       NAMES
30c629f1a12e   vrnetlab/vr-freebsd:13.2               "/launch.py --userna…"   23 hours ago   Up 23 hours (healthy)   22/tcp, 5000/tcp, 10000-10099/tcp           clab-freebsd01-fbsd1
6b476bfa41b1   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   23 hours ago   Up 23 hours             80/tcp, 443/tcp, 1180/tcp, 11443/tcp        clab-freebsd01-client2
21ab9e4857b3   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   23 hours ago   Up 23 hours             80/tcp, 443/tcp, 1180/tcp, 11443/tcp        clab-freebsd01-client1
```

### fbsd01

Log into the FreeBSD node using SSH with `ssh admin@clab-freebsd01-fbsd1` and add the following configuration. Password is `admin`.

```
sudo sysctl net.inet.ip.forwarding=1
sudo ifconfig vtnet1 192.168.1.1/30
sudo ifconfig vtnet2 192.168.2.1/30
```

### client1

The two clients should be configured with the correct IP addresses and a route to the other client via the FreeBSD node.
First attach to the container process `docker exec -it clab-freebsd01-client1 ash`

```
docker exec -it clab-freebsd01-client1 ash

# ip -br a show dev eth1
eth1@if71231     UP             192.168.1.2/30 fe80::a8c1:abff:fe17:b15f/64

# ip r
default via 172.20.20.1 dev eth0
172.20.20.0/24 dev eth0 proto kernel scope link src 172.20.20.3
192.168.1.0/30 dev eth1 proto kernel scope link src 192.168.1.2
192.168.2.0/30 via 192.168.1.1 dev eth1
```

## Verification

Traceroute from client1 to client2 to verify the data-plane via the FreeBSD node.

### client1

```
# traceroute 192.168.2.2
traceroute to 192.168.2.2 (192.168.2.2), 30 hops max, 46 byte packets
 1  192.168.1.1 (192.168.1.1)  0.680 ms  0.676 ms  0.597 ms
 2  192.168.2.2 (192.168.2.2)  1.105 ms  1.088 ms  0.869 ms
```
  
[freebsd]: https://freebsd.org/
[client]: https://github.com/wbitt/Network-MultiTool
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/freebsd01/freebsd01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
