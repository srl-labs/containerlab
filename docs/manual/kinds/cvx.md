---
search:
  boost: 4
kind_code_name: cumulus_cvx
kind_display_name: Cumulus VX
---
# -{{ kind_display_name }}-
[-{{ kind_display_name }}-](https://docs.nvidia.com/networking-ethernet-software/cumulus-vx/) is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).
The `cvx` kind defines a supported feature set and a startup procedure of a `cvx` node.


CVX nodes launched with containerlab come up with:

* the management interface `eth0` is configured with IPv4/6 addresses as assigned by either the container runtime or DHCP
* `root` user created with password `root`

!!! note
    Cumulus VX has been **discontinued** and the last version is v5.12.1. This change has been announced in the [Cumulus Linux 5.13 documentation](https://docs.nvidia.com/networking-ethernet-software/cumulus-linux-513/Whats-New/#cumulus-vx):
    > NVIDIA no longer releases Cumulus VX as a standalone image. To simulate a Cumulus Linux switch, use NVIDIA AIR.

## Mode of operation

CVX runs directly inside the container runtime (e.g. Docker or Podman). Due to the lack of Cumulus VX kernel modules, some features are not supported, most notably MLAG. To be explicit about the runtime, add `runtime: docker` under the cvx node definition (see also [this example](https://github.com/srl-labs/containerlab/blob/main/lab-examples/cvx02/topo.clab.yml)).

## Managing cvx nodes

Cumulus VX node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to attach to a `bash` shell of a running cvx container (only container ID is supported):
    ```bash
    docker attach <container-id>
    ```
    Use Docker's detach sequence (Ctrl+P+Q) to disconnect.

=== "SSH"
    SSH server is running on port 22
    ```bash
    ssh root@<container-name>
    ```
=== "gNMI"
    gNMI server will be added in future releases.

!!!info
    Default user credentials: `root:root`

### User-defined config

It is possible to make cvx nodes to boot up with a user-defined config by passing any number of files along with their desired mount path:

```yaml
name: cvx_lab
topology:
  nodes:
    cvx:
      kind: -{{ kind_code_name }}-
      binds:
        - cvx/interfaces:/etc/network/interfaces
        - cvx/daemons:/etc/frr/daemons
        - cvx/frr.conf:/etc/frr/frr.conf
```

## Lab examples

The following labs feature CVX node:

* [Cumulus and FRR](https://github.com/srl-labs/containerlab/blob/main/lab-examples/cvx01/topo.clab.yml)
* [Cumulus in Docker runtime and Host](https://github.com/srl-labs/containerlab/blob/main/lab-examples/cvx02/topo.clab.yml)
* [Cumulus Linux Test Drive](https://clabs.netdevops.me/rs/cvx03/)
* [EVPN with MLAG and multi-homing scenarios](https://clabs.netdevops.me/rs/cvx04/)
