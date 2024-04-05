|                               |                                                                        |
| ----------------------------- | ---------------------------------------------------------------------- |
| **Description**               | Two vr_linux (ubuntu) hosts connected to cEOS switch                   |
| **Components**                | [Ubuntu Hosts][ubuntu], [cEOS Switch][client]                          |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 1024 MB |
| **Topology file**             | [vr_linux01.yml][topofile]                                              |
| **Name**                      | vr-linux01                                                             |
| **Version information**[^2]   | `containerlab:`, `jammy-ubuntu-cloud.qcow2`, `docker:24.0.6`            |

## Description

This lab consists of two vr_linux (ubuntu) nodes connected to a cEOS switch.

```
ubuntu-01<---->cEOS<---->ubuntu-02
```

## Configuration

The linux vm nodes takes about 1 minute to complete its start up. Check using "docker container ls" until the linux containers shows up as "healthy".

```
# docker container ls
CONTAINER ID   IMAGE                      COMMAND                  CREATED          STATUS                    PORTS                               NAMES
0c459aed936c   vrnetlab/vr-ubuntu:jammy   "/launch.py --userna…"   5 minutes ago    Up 5 minutes (healthy)    22/tcp, 5000/tcp, 10000-10099/tcp   clab-vr-linux01-ubuntu-01
b0e18a291608   ceos:4.31.2F               "bash -c '/mnt/flash…"   5 minutes ago    Up 5 minutes                                                  clab-vr-linux01-ceos
38a3c825fcac   vrnetlab/vr-ubuntu:jammy   "/launch.py --userna…"   5 minutes ago    Up 5 minutes (healthy)    22/tcp, 5000/tcp, 10000-10099/tcp   clab-vr-linux01-ubuntu-02
```

### ubuntu-01

Log into the ubuntu-01 node using SSH with `ssh sysadmin@clab-vr-linux01-ubuntu-01` and add the following configuration. Password is `sysadmin`.

```
sudo ip addr add dev enp1s2 192.168.1.1/24
sudo ip link set dev enp1s2 up
```

### ubuntu-02

Log into the ubuntu-01 node using SSH with `ssh sysadmin@clab-vr-linux01-ubuntu-02` and add the following configuration. Password is `sysadmin`.

```
sudo ip addr add dev enp1s2 192.168.1.2/24
sudo ip link set dev enp1s2 up
```

### ceos

The cEOS node is used as a switch, there is no additional configuration needed.

## Verification

Ping from ubuntu-01 to ubuntu-02 to verify the connectivity.

### ubuntu-01

```
# ping 192.168.1.2
PING 192.168.1.2 (192.168.1.2) 56(84) bytes of data.
64 bytes from 192.168.1.2: icmp_seq=1 ttl=64 time=1.80 ms
64 bytes from 192.168.1.2: icmp_seq=2 ttl=64 time=1.65 ms
64 bytes from 192.168.1.2: icmp_seq=3 ttl=64 time=1.59 ms
^C
--- 192.168.1.2 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2003ms
rtt min/avg/max/mdev = 1.594/1.679/1.799/0.087 ms
```
  
[ubuntu]: https://ubuntu.com/
[ceos]:
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/vr-linux01/vr-linux01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
