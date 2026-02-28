|                               |                                                                                        |
| ----------------------------- | -------------------------------------------------------------------------------------- |
| **Description**               | BNG with subscriber simulator and core router                                          |
| **Components**                | [Code Laboratory BNG][bng], [BNG Blaster][blaster], [FRR][frr]                         |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                   |
| **Topology file**             | [bng01.clab.yml][topofile]                                                             |
| **Name**                      | bng01                                                                                  |

## Description

This lab demonstrates Code Laboratory's eBPF/XDP-based BNG handling IPoE/DHCPv4 subscriber sessions. The topology consists of three nodes:

| Node | Role | Image |
|------|------|-------|
| `bng1` | Broadband Network Gateway | `ghcr.io/codelaboratoryltd/bng:latest` |
| `subscribers` | Subscriber simulator (BNG Blaster) | `veesixnetworks/bngblaster:0.9.30` |
| `corerouter1` | Core/upstream router (FRR) | `frrouting/frr:v8.4.1` |

## Topology

```
subscribers:eth1 <-----> bng1:eth1 (access)
                         bng1:eth2 (core) <-----> corerouter1:eth1
```

## Deploying the lab

```bash
sudo clab deploy -t lab-examples/bng01/bng01.clab.yml
```

## Verification

Check BNG is running:

```bash
docker logs clab-bng01-bng1
```

Check FRR routing:

```bash
docker exec clab-bng01-corerouter1 vtysh -c "show ip route"
```

Run BNG Blaster to simulate 10 untagged IPoE/DHCPv4 subscribers:

```bash
docker exec -it clab-bng01-subscribers bngblaster -C /config/config.json
```

A QinQ (802.1ad) subscriber config is also provided for double-tagged deployments:

```bash
docker exec -it clab-bng01-subscribers bngblaster -C /config/qinq.json
```

Check BNG metrics:

```bash
curl http://clab-bng01-bng1:9090/metrics
```

## Cleanup

```bash
sudo clab destroy -t lab-examples/bng01/bng01.clab.yml
```

[bng]: https://github.com/codelaboratoryltd/bng
[blaster]: https://github.com/rtbrick/bngblaster
[frr]: https://docs.frrouting.org/en/stable-8.4/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/bng01/bng01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
