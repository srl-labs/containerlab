|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | A 3-node ring of FRR routers with OSPF IGP                           |
| **Components**                | [FRR](https://docs.frrouting.org/en/stable-10.7/overview.html)       |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [frr01.clab.yml][topofile]                                           |
| **Name**                      | frr01                                                                |
| **Version information**[^2]   | `containerlab:0.71.0`, `quay.io/frrouting/frr:containerlab-10.7.1`, `docker-ce:28.5.2` |

## Description

This lab example consists of three FRR routers connected in a ring topology, running OSPF between them. Each router has one PC connected to it, reachable over static routes.

The routers use the [`frr`](../manual/kinds/frr.md) kind. Each one gets its configuration from a `startup-config` file, which containerlab renders into the node's `/etc/frr/frr.conf`:

```yaml
router1:
  group: routers
  startup-config: router1/frr.conf
```

The lab only runs OSPF, so the `routers` group narrows the daemons down from the default of all of them:

```yaml
groups:
  routers:
    kind: frr
    image: quay.io/frrouting/frr:containerlab-10.7.1
    extras:
      frr:
        daemons:
          - ospfd
```

Setting this on the group rather than on each node means all three routers share the list. See [Daemons](../manual/kinds/frr.md#daemons) for the full set of names.

To start the lab:

```bash
containerlab deploy -t frr01.clab.yml
```

The PCs configure their own interfaces and routes through the `exec` statements in the topology file, so the lab needs no further setup.

## Verification

Check that OSPF has come up on each router:

```bash
ssh root@clab-frr01-router1 'vtysh -c "show ip ospf neighbor"'
```

Each router should have two neighbours in the `Full` state. The PCs can then reach each other's networks:

```bash
docker exec clab-frr01-PC1 ping -c3 192.168.13.2
```

The lab configuration is documented in detail at: https://www.brianlinkletter.com/2021/05/use-containerlab-to-emulate-open-source-routers/

[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/frr01/frr01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
