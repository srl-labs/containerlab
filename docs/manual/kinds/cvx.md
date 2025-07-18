---
search:
  boost: 4
---
# Cumulus VX

[Cumulus VX](https://docs.nvidia.com/networking-ethernet-software/cumulus-vx/) is identified with `cvx` or `cumulus_cvx` kind in the [topology file](../topo-def-file.md). The `cvx` kind defines a supported feature set and a startup procedure of a `cvx` node.

CVX nodes launched with containerlab come up with:

* the management interface `eth0` is configured with IPv4/6 addresses as assigned by either the container runtime or DHCP
* `root` user created with password `root`

!!! note
    Cumulus VX has been **discontinued** and the last version is v5.12.1. This change has been announced in the [Cumulus Linux 5.13 documentation](https://docs.nvidia.com/networking-ethernet-software/cumulus-linux-513/Whats-New/#cumulus-vx):
    > NVIDIA no longer releases Cumulus VX as a standalone image. To simulate a Cumulus Linux switch, use NVIDIA AIR.

## Mode of operation

CVX supports two modes of operation:

* Using only the container runtime -- this mode runs Cumulus VX container image directly inside the container runtime (e.g. Docker). Due to the lack of Cumulus VX kernel modules, some features are not supported, most notable one being MLAG. In order to use this mode, add `runtime: docker` under the cvx node definition (see also [this example](https://github.com/srl-labs/containerlab/blob/main/lab-examples/cvx02/topo.clab.yml)).
* Using Firecracker micro-VMs -- this mode runs Cumulus VX inside a micro-VM on top of the native Cumulus kernel. This mode uses `ignite` runtime and is the default way of running CVX nodes.

    !!!warning
        This mode was broken in containerlab between v0.27.1 and v0.32.1 due to dependencies issues in ignite[^2].

!!! note
    When running in the default `ignite` runtime mode, the only host OS dependency is `/dev/kvm`[^1] required to support hardware-assisted virtualisation. Firecracker VMs are spun up inside a special "sandbox" container that has all the right tools and dependencies required to run micro-VMs.

    Additionally, containerlab creates a number of directories under `/var/lib/firecracker` for nodes running in `ignite` runtime to store runtime metadata; these directories are managed by containerlab.

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
      kind: cvx
      binds:
        - cvx/interfaces:/etc/network/interfaces
        - cvx/daemons:/etc/frr/daemons
        - cvx/frr.conf:/etc/frr/frr.conf
```

### Configuration persistency

When running inside the `ignite` runtime, all mount binds work one way -- from host OS to the cvx node, but not the other way around. Currently, it's up to a user to manually update individual files if configuration updates need to be persisted.
This will be addressed in the future releases.

## Lab examples

The following labs feature CVX node:

* [Cumulus and FRR](https://github.com/srl-labs/containerlab/blob/main/lab-examples/cvx01/topo.clab.yml)
* [Cumulus in Docker runtime and Host](https://github.com/srl-labs/containerlab/blob/main/lab-examples/cvx02/topo.clab.yml)
* [Cumulus Linux Test Drive](https://clabs.netdevops.me/rs/cvx03/)
* [EVPN with MLAG and multi-homing scenarios](https://clabs.netdevops.me/rs/cvx04/)

## Known issues or limitations

* CVX in Ignite is always attached to the default docker bridge network

[^1]: this device is already part of the linux kernel, therefore this can be read as "no external dependencies are needed for running cvx with `ignite` runtime".
[^2]: see <https://github.com/srl-labs/containerlab/pull/1037> and <https://github.com/srl-labs/containerlab/issues/1039>
