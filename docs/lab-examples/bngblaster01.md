|                               |                                                                                    |
| ----------------------------- | ---------------------------------------------------------------------------------- |
| **Description**               | RtBrick BNG Blaster emulating PPPoE server and clients back to back               |
| **Components**                | [RtBrick BNG Blaster](../manual/kinds/rtbrick_bngblaster.md)                       |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 1 GB               |
| **Topology file**             | [bngblaster01.clab.yml][topofile]                                                  |
| **Name**                      | bngblaster01                                                                       |

## Description

A single BNG Blaster node with `eth1` connected to `eth2`. The A10NSP interface `eth1` emulates a PPPoE server and `eth2` the PPPoE clients, so the lab needs no device under test. Replace one side with a BNG to test it.

The [startup-config](../manual/kinds/rtbrick_bngblaster.md#startup-configuration) `pppoe.json` establishes 10 PPPoE sessions with session traffic and traffic streams, and the test starts automatically once both links are attached. `run.json` enables the report with session and stream details and logging.

## Use cases

Follow the test while it is running:

```bash
docker exec clab-bngblaster01-bngblaster \
  bngblaster-cli /var/bngblaster/clab/run.sock session-counters
```

or open the web UI at `http://clab-bngblaster01-bngblaster:8001`.

When the lab is destroyed, the test is stopped and its report is written to `clab-bngblaster01/bngblaster/clab/run_report.json`.

[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/bngblaster01/bngblaster01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
