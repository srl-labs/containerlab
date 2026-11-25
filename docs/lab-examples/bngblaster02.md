|                               |                                                                                    |
| ----------------------------- | ---------------------------------------------------------------------------------- |
| **Description**               | RtBrick BNG Blaster emulating an IS-IS topology towards FRR, verified with traffic |
| **Components**                | [RtBrick BNG Blaster](../manual/kinds/rtbrick_bngblaster.md), [FRR](../manual/kinds/frr.md) |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 1 GB               |
| **Topology file**             | [bngblaster02.clab.yml][topofile]                                                  |
| **Name**                      | bngblaster02                                                                       |

## Description

The BNG Blaster forms an IS-IS level 1 adjacency with FRR on `eth1` and advertises an emulated topology of 20 routers behind it, generated with [lspgen](https://rtbrick.github.io/bngblaster/routing/lspgen.html) and loaded from `isis.mrt`. FRR learns the routes to all emulated routers.

Two traffic streams are sent from the second BNG Blaster interface `eth2` to the loopback addresses of the emulated routers `node1` (192.168.0.0) and `node20` (192.168.0.11). FRR routes them back to the BNG Blaster on `eth1`, which verifies the streams for loss. The packets sent before FRR installed the routes, usually within a few seconds after the start, are not received.

```
bngblaster eth1 <---- IS-IS adjacency ----> eth1 frr
bngblaster eth2 ---- traffic streams -----> eth2 frr
```

The test starts automatically once both links are attached.

## Use cases

The IS-IS database and routes of FRR:

```bash
docker exec clab-bngblaster02-frr vtysh -c "show isis database"
docker exec clab-bngblaster02-frr vtysh -c "show ip route isis"
```

The adjacency and the traffic streams on the BNG Blaster:

```bash
docker exec clab-bngblaster02-bngblaster \
  bngblaster-cli /var/bngblaster/clab/run.sock isis-adjacencies
docker exec clab-bngblaster02-bngblaster \
  bngblaster-cli /var/bngblaster/clab/run.sock stream-summary
```

When the lab is destroyed, the BNG Blaster is stopped first, withdraws its IS-IS topology and closes the adjacency before FRR and the links are removed, and the stream results are written to `clab-bngblaster02/bngblaster/clab/run_report.json`.

FRR pads its IS-IS hellos to the interface MTU, which is 9500 for containerlab links by default, and sizes its CSNP and PSNP according to it as well. `isis.json` therefore enables `jumbo-frames`, sets the `mtu` of `eth1` to 9500 and raises the IS-IS `max-pdu-size` to 9497 (MTU minus 3 bytes for the LLC header). IS-IS PDUs larger than 1497 bytes are sent as LLC in Ethernet II frames with ethertype 0x8870, see [IS-IS jumbo frames](https://rtbrick.github.io/bngblaster/routing/isis.html#jumbo-frames). This requires BNG Blaster 1.2.0 or newer.

[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/bngblaster02/bngblaster02.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
