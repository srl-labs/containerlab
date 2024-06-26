# Ostinato and Nokia SR Linux

|                               |                                                                                    |
| ----------------------------- | ---------------------------------------------------------------------------------- |
| **Description**               | Ostinato traffic generator connected with Nokia SR Linux                           |
| **Components**                | [Ostinato][ostinato], [Nokia SR Linux][srl]                                        |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 4 GB               |
| **Topology file**             | [ost-srl.clab.yaml][topofile]                                                      |
| **Name**                      | ost-srl                                                                            |
| **Version information**[^2]   | `containerlab:0.55.1`, `ostinato:v1.3.0-1`, `srlinux:24.3.2`, `docker-ce:26.0.0` |

## Description

This lab consists of a [Ostinato](../manual/kinds/ostinato.md) node with 2 ports connected to 2 ports on a Nokia SR Linux node via two point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

[Ostinato][ostinato] is a software based network packet traffic generator managed via a GUI or a Python script using the [Ostinato API][ostinato-api]. This example will demonstrate how to use the Ostinato GUI included with the Ostinato for Containerlab image to verify IPv4 forwarding.

<div class='mxgraph' style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;' data-mxgraph='{"page":0,"zoom":2,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank","url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ost-srl-clab.drawio"}'></div>

## Deployment

Change into the lab directory:

```Shell
cd containerlab/lab-examples/ost-srl
```

Deploy the lab:

```Shell
sudo containerlab deploy
```

## Use cases

This lab allows users to validate IPv4 forwarding on Nokia SR Linux (the DUT) using Ostinato as the traffic generator.

### IPv4 Traffic forwarding

<div class='mxgraph' style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;' data-mxgraph='{"page":1,"zoom":1.5,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank","url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ost-srl-clab.drawio"}'></div>

This lab demonstrates a simple IPv4 traffic forwarding scenario where

- Ostinato with two test ports `eth1` and `eth2` connected to Nokia SR Linux ports `e1-1` and `e1-2` respectively.
- SR Linux is the DUT and its interfaces `e1-1` and `e1-2` are configured with IPv4 addresses `10.0.0.1/24` and `20.0.0.1/24` respectively.
- Ostinato will emulate host `10.0.0.100` on `eth1` and `20.0.0.100` on `eth2`
- Ostinato will send bidirectional traffic at 100pps (`eth1` --> `e1-1`) and 200pps (`eth2` --> `e1-2`)
- The TX and RX traffic rates can be verified in the Port Stats window of the Ostinato GUI

#### Configuration

During the lab deployment and test execution the following configuration is applied to the lab nodes to forward and receive traffic.

- **SR Linux**  
    SR Linux node comes up pre-configured with the commands listed in [srl.cfg][srlcfg] file which configure IPv4 addresses on both interfaces.

- **Ostinato**  
    Ostinato configuration is saved in the [ost-srl.ossn][ostcfg] file which will configure the emulated hosts and traffic streams. The configuration file needs to be loaded manually as explained in the next section

/// admonition
    type: warning
All Ostinato stream and session files are NOT in text (or human readable) format but a binary format read and written by Ostinato
///

#### Execution

1. Access the Ostinato GUI by connecting a VNC client to `<host-ip>:5900`
1. Once the GUI opens and the `eth1`, `eth2` ports are listed, go to `File | Open Session` and open `/root/shared/ost-srl.ossn`
1. In the Ostinato Port Stats window, select both `eth1` and `eth2` port columns and click on :material-play: (_start transmit_)

#### Verification

1. Verify the port stats and rates in the same window
1. Keeping both `eth1` and `eth2` selected, click on :material-stop: (_stop transmit_)

Here's a short video showing the above steps -

<div class="iframe-container">
<iframe width="100%" src="https://www.youtube.com/embed/KHUTuL7fc2I" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
</div>

#### Next Steps

You can edit the Ostinato emulated devices and streams to experiment further.

Here are some suggestions -

- Override the IPv4 checksum to an invalid value and verify the DUT silently discards the packets (some DUTs may send an ICMP parameter problem)
- Try setting IPv4 TTL to 1 and verify
    - traffic is not forwarded by the DUT
    - DUT sends back ICMP TTL Exceeded (you can capture and view captured packets)
- Try IPv6 traffic streams

Learn more about [Ostinato traffic streams][ostinato-streams].

## Cleanup

To stop the lab, use:

```Shell
sudo containerlab destroy --cleanup
```

[ostinato]: https://ostinato.org/
[ostinato-api]: https://apiguide.ostinato.org/tutorial/
[ostinato-streams]: https://userguide.ostinato.org/stream-config/
[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ost-srl/ost-srl.clab.yml
[srlcfg]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ost-srl/srl.cfg
[ostcfg]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ost-srl/ost-srl.ossn

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
