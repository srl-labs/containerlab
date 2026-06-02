|                               |                                                                                              |
| ----------------------------- | -------------------------------------------------------------------------------------------- |
| **Description**               | A BNG topology with osvbng, BNG Blaster subscriber simulator, and FRR core router            |
| **Components**                | [v::n osvbng][osvbng], [BNG Blaster][bngblaster], [FRR][frr]                                 |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                         |
| **Topology file**             | [osvbng01.clab.yml][topofile]                                                                |
| **Name**                      | osvbng01                                                                                     |
| **Version information**[^2]   | `veesixnetworks/osvbng:v0.3.1`, `veesixnetworks/bngblaster:0.9.30`, `frrouting/frr:v8.4.1`  |

## Description

This lab demonstrates a minimal BNG (Broadband Network Gateway) topology using [v::n osvbng](../manual/kinds/veesix_osvbng.md) for subscriber termination, [BNG Blaster](https://github.com/rtbrick/bngblaster) as a subscriber traffic simulator, and [FRR](https://frrouting.org/) as the core router.

The topology simulates a real-world broadband access network with QinQ IPoE subscriber sessions using DHCPv4 and DHCPv6.

```
subscribers:eth1 <----> bng1:eth1 (access)
                        bng1:eth2 (core) <----> corerouter1:eth1
```

- **subscribers** - BNG Blaster container simulating subscribers with Q-in-Q tagged IPoE sessions over DHCPv4/DHCPv6
- **bng1** - osvbng node performing subscriber termination with OSPF/OSPFv3 towards the core
- **corerouter1** - FRR router acting as the core/upstream router

## Deployment

Deploy the lab:

```bash
sudo containerlab deploy -t osvbng01.clab.yml
```

Wait a few seconds for osvbng to boot and establish OSPF adjacency, then verify:

```bash
docker exec clab-osvbng01-corerouter1 vtysh -c "show ip ospf neighbor"
```

You should see `bng1` as a Full neighbor.

## Running BNG Blaster

Start BNG Blaster to simulate subscriber sessions:

```bash
docker exec -it clab-osvbng01-subscribers bngblaster -C /config/config.json
```

BNG Blaster will establish IPoE sessions with DHCPv4 and DHCPv6 and report session status:

```
Sessions PPPoE: 0 IPoE: 1
Sessions established: 1/1
DHCPv6 sessions established: 1
```

## Cleanup

Destroy the lab:

```bash
sudo containerlab destroy -t osvbng01.clab.yml
```

[osvbng]: ../manual/kinds/veesix_osvbng.md
[bngblaster]: https://github.com/rtbrick/bngblaster
[frr]: https://frrouting.org/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/osvbng01/osvbng01.clab.yml

[^1]: Resource requirements are provisional. Consult with the osvbng documentation for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
