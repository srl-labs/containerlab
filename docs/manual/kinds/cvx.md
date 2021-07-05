# Cumulus VX

[Cumulus VX](https://docs.nvidia.com/networking-ethernet-software/cumulus-vx/) is identified with `cvx` kind in the [topology file](../topo-def-file.md). The `cvx` kind defines a supported feature set and a startup procedure of a `cvx` node.

CVX nodes launched with containerlab comes up with:

* the management interface `eth0` configured with IPv4/6 addresses as assigned by either the container runtime or DHCP
* `root` user created with password `root`

## Mode of operation

CVX supports two modes of operation:

* Using Firecracker micro-VMs -- this mode runs Cumulus VX inside a micro-VM on top of the native Cumulus kernel. This is the default way of running CVX nodes.
* Using only the container runtime -- this mode runs Cumulus VX container image directly inside the container runtime (e.g. Docker). Due to the lack of Cumulus VX kernel modules, some features are not supported, most notable one being MLAG. In order to use this mode add `runtime: docker` under the cvx node definition (see also [this example](../../lab-examples/cvx02.md)).


## Managing cvx nodes
Cumulus VX node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to attach to a `bash` shell of a running cvx container (only container ID is supported):
    ```bash
    docker attach <container-id> 
    ```
    Use Docker's detach sequence (^P^Q) to disconnect.

=== "SSH"
    SSH server is running on port 22
    ```bash
    ssh root@<container-name> 
    ```
=== "gNMI"
    gNMI server will be added in future releases.
    

!!!info
    Default user credentials: `root:root`

## Node definition
In order to run CVX nodes, 


#### User defined config
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



### Note on configuration persistency

When running inside the `ignite` runtime, all mount binds work one way -- from host OS to the cvx node, but not the other way around. Currently, it's up to a user to manually update individual files if configuration updates need to be persisted.
This will be addressed in the future releases.


## Lab examples
The following labs feature CVX node:

- [Cumulus Linux Test Drive](../../lab-examples/cvx03.md)
- [Cumulus and FRR](../../lab-examples/cvx01.md)


## Known issues or limitations

* CVX in Ignite is always be attache to the default docker bridge network