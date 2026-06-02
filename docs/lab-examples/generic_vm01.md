|                               |                                                                         |
| ----------------------------- | ----------------------------------------------------------------------- |
| **Description**               | A generic Ubuntu VM interconnected with Nokia SR Linux                  |
| **Components**                | [Ubuntu VM][ubuntu], [SR Linux][srl]                                    |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 4096 MB |
| **Topology file**             | [generic_vm01.yml][topofile]                                            |
| **Version information**[^2]   | `containerlab:0.55.0`, `ubuntu:22.04`, `docker:26.0.0`                  |

## Description

This lab demonstrates how to use a [Generic VM kind](../manual/kinds/generic_vm.md) using Ubuntu 22.04 LTS by connecting it to the SR Linux switch and running a basic ping test.

The topology is rather simple, with Ubuntu VM and SR Linux switch connected over a single interface.

```
ubuntu:eth1 <----> e1-1:SR Linux
```

## Deployment

Before deploying the lab, make sure you have built the container image for the Ubuntu VM using [srl-labs/vrnetlab](https://github.com/srl-labs/vrnetlab/tree/master/ubuntu) project. The topology file references the image as `vrnetlab/vr-ubuntu:jammy`, which is the default image name for Ubuntu 22.04 LTS.

After building the image, deploy the lab using the following command:

```bash
sudo containerlab deploy -t generic_vm.clab.yml
```

## Configuration

The Ubuntu 22.04 VM takes about 1 minute to complete its start up and then extra 30 seconds to allow password-based authentication over SSH. Check the boot log using `docker logs -f clab-generic_vm-ubuntu`.

### SR Linux

As seen in the topology file, the SR Linux node comes with its `ethernet-1/1` interface and subinterface preconfigured with `192.168.0.2/24` IP address. Thus no additional configuration is needed.

### ubuntu

Log into the `ubuntu` node using SSH with `ssh clab-generic_vm-ubuntu` and add the IP configuration to the `ens2` interface that connects the VM with SR Linux switch. Password is `clab@123`.

```
sudo ip addr add dev ens2 192.168.0.1/24
sudo ip link set dev ens2 up
```

## Verification

With interface on Ubuntu side configured, ping from Ubuntu to SR Linux to verify the connectivity.

```
clab@ubuntu:~$ ping 192.168.0.2
PING 192.168.0.2 (192.168.0.2) 56(84) bytes of data.
64 bytes from 192.168.0.2: icmp_seq=1 ttl=64 time=14.2 ms
64 bytes from 192.168.0.2: icmp_seq=2 ttl=64 time=8.85 ms
^C
--- 192.168.0.2 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1002ms
rtt min/avg/max/mdev = 8.849/11.508/14.168/2.659 ms
```
  
[ubuntu]: https://ubuntu.com/
[srl]: ../manual/kinds/srl.md
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/generic_vm01/generic_vm.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
