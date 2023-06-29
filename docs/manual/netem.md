# Link Impairment

Being able to introduce link impairments such as delay, jitter, and packet loss, is quite a useful feature for testing network applications. It allows testing the application behavior under different network conditions and simulate different network properties. For example, making a network link more lossy can be used to simulate a wireless or congested link, while adding delay and jitter can be used to simulate a satellite link.

The Linux kernel has a built-in traffic control tool called `tc` which can be used to configure network emulation. The `tc` tool is powerful, but it can be difficult to use, especially when links you want to apply impairments to belong to a container process.

To simplify the process of configuring network emulation, we are providing a small shell script that orchestrates the [`alexei-led/pumba`](https://github.com/alexei-led/pumba) tool that applies different network impairments to containeres' links.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:13,&quot;zoom&quot;:2,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/impairments.drawio&quot;}"></div>

Link impairments can be applied to any container link, as the diagram above shows. This allows to granularly control the network conditions for each link in the topology and implement complex network impairment scenarios. One caveat with the current implementation is that users have to use another tool for adding impairments to the Linux bridges, see [Linux bridge](#linux-bridge) section for more details.

## Pre-requisites

In the spirit of containerlab, we try to use containerized applications whenever possible. The [`netem.sh` script](#netemsh-script) that we use to orchestrate link impairments uses containerized tools, so there are no pre-requisites to install on the host machine.

### Lab setup

We will demonstrate the usage of the [`netem.sh` script](#netemsh-script) on a simple topology with two Linux containers connected back-to-back over their `eth1` interfaces. The following topology file will be used:

```yaml
name: netem
topology:
  nodes:
    r1:
      kind: linux
      image: alpine:3
      exec:
        - ip addr add 192.168.0.1/30 dev eth1
    r2:
      kind: linux
      image: alpine:3
      exec:
        - ip addr add 192.168.0.2/30 dev eth1
  links:
    - endpoints: ["r1:eth1", "r2:eth1"]
```

When the lab is deployed, we can ensure that `r1` can ping `r2` and the reported round-trip time corresponds to an unaffected direct link:

```
❯ docker exec -it clab-netem-r1 ping 192.168.0.2
PING 192.168.0.2 (192.168.0.2): 56 data bytes
64 bytes from 192.168.0.2: seq=0 ttl=64 time=0.086 ms
64 bytes from 192.168.0.2: seq=1 ttl=64 time=0.086 ms
64 bytes from 192.168.0.2: seq=2 ttl=64 time=0.064 ms
^C
--- 192.168.0.2 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 0.064/0.078/0.086 ms
```

Users are free to use any other topology for their experiments if desired.

### Get the `netem.sh` script

The [`netem.sh`](https://github.com/srl-labs/containerlab/blob/main/docs/manual/netem.sh) script is a wrapper around the [`pumba`](https://github.com/alexei-led/pumba) tool that allows to apply different network impairments to container links. It is meant to be customized by the users while having some basic impairments readily available.

???tip "Script content"
    ```bash
    --8<-- "docs/manual/netem.sh"
    ```

Feel free to either copy the contents of the script to your own file or download it from the repository.

```bash
curl -LO https://raw.githubusercontent.com/srl-labs/containerlab/main/docs/manual/netem.sh
chmod +x netem.sh
```

## Usage

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
