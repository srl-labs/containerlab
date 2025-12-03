|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | A Cisco ASAv connected to two Alpine Linux Hosts                     |
| **Components**                | [Cisco ASAv][asav], [Multitool Alpine Linux][client]                 |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [asav01.clab.yml][topofile]                                          |
| **Name**                      | asav01                                                               |
| **Version information**[^2]   | `asav9-23-1.qcow2`, `docker:24.0.6`                                  |

## Description

This lab consists of one Cisco ASAv firewall connected to two Alpine Linux nodes.

```
client1<---->ASAv<---->client2
```

## Configuration

The ASAv node takes about 5-7 minutes to complete its start up. Check using "docker container ls" and "docker logs -f clab-asav01-asav1" until the ASAv container shows up as "healthy".

```
# docker container ls
CONTAINER ID   IMAGE                                  COMMAND                  CREATED          STATUS                    PORTS                                       NAMES
5682d73984d1   vrnetlab/vr-asav:9.23.1                "/launch.py --userna…"   5 minutes ago    Up 5 minutes (healthy)    22/tcp, 80/tcp, 443/tcp, 5000/tcp, 10000-10099/tcp   clab-asav01-asav1
1ebe3dae6846   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   5 minutes ago    Up 5 minutes              80/tcp, 443/tcp, 1180/tcp, 11443/tcp                 clab-asav01-client1
9726c9bb9e21   wbitt/network-multitool:alpine-extra   "/bin/sh /docker-ent…"   5 minutes ago    Up 5 minutes              80/tcp, 443/tcp, 1180/tcp, 11443/tcp                 clab-asav01-client2
```

### asav1

Log into the ASAv node using SSH and add the following configuration. Password is `CiscoAsa1!`.

```bash
ssh admin@clab-asav01-asav1
```

Optionally configure the ASA with any additional settings as needed.

### client1

The two clients should be configured with the correct IP addresses and a route to the other client via the ASAv node.
First attach to the container process `docker exec -it clab-asav01-client1 bash`

```
docker exec -it clab-asav01-client1 bash

# ip -br a show dev eth1
eth0@if7         UP             172.20.20.4/24 3fff:172:20:20::4/64 fe80::a4ea:64ff:fe33:c15c/64

# ip route
default via 172.20.20.1 dev eth0
172.20.20.0/24 dev eth0 proto kernel scope link src 172.20.20.4

# ping 172.20.20.2
PING 172.20.20.2 (172.20.20.2) 56(84) bytes of data.
64 bytes from 172.20.20.2: icmp_seq=1 ttl=64 time=0.163 ms
64 bytes from 172.20.20.2: icmp_seq=2 ttl=64 time=0.047 ms
64 bytes from 172.20.20.2: icmp_seq=3 ttl=64 time=0.053 ms
```

### client2

Similarly for client2, verify connectivity:

```
docker exec -it clab-asav01-client2 bash

# ip -br a show dev eth1
eth0@if5         UP             172.20.20.2/24 3fff:172:20:20::2/64 fe80::b86b:51ff:fed8:1c85/64

# ping 172.20.20.4
PING 172.20.20.4 (172.20.20.4) 56(84) bytes of data.
64 bytes from 172.20.20.4: icmp_seq=1 ttl=64 time=0.055 ms
64 bytes from 172.20.20.4: icmp_seq=2 ttl=64 time=0.035 ms
64 bytes from 172.20.20.4: icmp_seq=3 ttl=64 time=0.065 ms

# ping 172.20.20.6
PING 172.20.20.6 (172.20.20.6) 56(84) bytes of data.
From 172.20.20.2 icmp_seq=1 Destination Host Unreachable
From 172.20.20.2 icmp_seq=2 Destination Host Unreachable
From 172.20.20.2 icmp_seq=3 Destination Host Unreachable
```

[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/asav01/asav01.clab.yml
