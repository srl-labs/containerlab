Every lab must have a packet capturing abilities, without it data plane verification becomes unnecessary complicated.

<p align=center>
<img src="https://gitlab.com/rdodin/pics/-/wikis/uploads/a7bffdae0393c9de41545c627f9b9f30/wsh.png" width="40%"></img>
</p>

Containerlab is no exception and capturing packets is something you can and should do with the labs launched by containerlab.

Consider the following lab topology which highlights the typical points of packet capture.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:13,&quot;zoom&quot;:2,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>

Since containerlab leverages linux network devices, users are free to use whatever tool of choice to sniff from any of them. This article will provide examples for `tcpdump` and `wireshark` tools.

## Packet capture, namespaces and interfaces
Capturing the packets from an interface requires having that interface name and it's network namespace (netns). And that's it.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlceos01.drawio&quot;}"></div>

Keep in mind, that containers employ network isolation by the means of network namespaces. As depicted above, each container has its own network namespace which is named exactly the same. This makes it trivial to pinpoint which namespace to use.

If containerlab at the end of a lab deploy reports that it created the containers with the names

- clab-lab1-srl
- clab-lab1-ceos
- clab-lab1-linux

then the namespaces for each of those containers will be named the same (clab-lab1-srl, etc).

To list the interfaces (links) of a given container leverage the `ip` utility:

```bash
# where $netns_name is the container name of a node
ip netns exec $netns_name ip link
```

## Capturing with tcpdump/wireshark
Now when it is clear which netns names corresponds to which container and which interfaces are available inside the given lab node, its extremely easy to start capturing traffic.

### local capture
From the containerlab host to capture from any interface inside a container simply use:

```bash
# where $lab_node_name is the name of the container, which is also the name of the network namespace
# and $if_name is the interface name inside the container netns
ip netns exec $lab_node_name tcpdump -nni $if_name
```

### remote capture
If you want to start capture from a remote machine, then add `ssh` command to the mix:

```bash
ssh $containerlab_host_address "ip netns exec $lab_node_name tcpdump -nni $if_name"
```

Capturing remotely with `tcpdump` makes little sense, but it makes all the difference when `wireshark` is concerned.

Wireshark normally is not installed on the containerlab host, but it more often than not installed on the users machine/laptop. Thus it is possible to use remote capture capability to let wireshark receive the traffic from the remote containerlab node:


```bash
ssh $containerlab_host_address "ip netns exec $lab_node_name tcpdump -U -nni $if_name -w -" | wireshark -k -i -
```

This will start the capture from a given interface and redirect the received flow to the wireshark input.

<video width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/6a0b8fb25d46b3764e8b2ce4667c07f7/wireshark.mp4" type="video/mp4">
</video>

!!!note
    Windows users should use WSL and invoke the command similar to the following:
    ```bash
    ssh $containerlab_host_address "ip netns exec $lab_node_name tcpdump -U -nni $if_name -w -" | /mnt/c/Program\ Files/Wireshark/wireshark.exe -k -i -
    ```

## Examples
Lets take the first diagram of this article and see which commands are used to sniff from the highlighted interfaces.

In the examples below the wireshark will be used as a sniffing tool and the following naming simplifications and conventions used:

* `$clab_host` - address of the containerlab host
* `clab-pcap-srl`, `clab-pcap-ceos`, `clab-pcap-linux` - container names of the SRL, cEOS and Linux nodes accordingly.

=== "SR Linux [1], [4]"
    SR Linux linecard interfaces are named as `e<linecard_num>-<port_num>` which translates to `ethernet-<linecard_num>/<port_num>` name inside the NOS itself.  
    So to capture from `ethernet-1/1` interface the following command should be used:
    ```bash
    ssh $clab_host "ip netns exec $clab-pcap-srl tcpdump -U -nni e1-1 -w -" | wireshark -k -i -
    ```
    The management interface on the SR Linux container is named `mgmt0`, so the relevant command will look like:
    ```bash
    ssh $clab_host "ip netns exec $clab-pcap-srl tcpdump -U -nni mgmt0 -w -" | wireshark -k -i -
    ```
=== "cEOS [2]"
    Similarly to SR Linux example, to capture the data interface of cEOS is no different. Just pick the right interface:
    ```bash
    ssh $clab_host "ip netns exec $clab-pcap-ceos tcpdump -U -nni eth1 -w -" | wireshark -k -i -
    ```
=== "Linux container [3]"
    A bare linux container is no different, its interfaces are named `ethX` where `eth0` is the interface connected to the containerlab management network.  
    So to capture from the first data link we will use `eth1` interface:
    ```bash
    ssh $clab_host "ip netns exec $clab-pcap-linux tcpdump -U -nni eth1 -w -" | wireshark -k -i -
    ```
=== "management bridge [5]"
    It is also possible to listen for all management traffic that traverses the containerlab's management network. To do that you firstly need to [find out the name of the linux bridge](network.md#connection-details) and then capture from it:
    ```bash
    ssh $clab_host "tcpdump -U -nni brXXXXXX -w -" | wireshark -k -i -
    ```
    Note that in this case you do not need to drill into the network namespace, since management bridge is in the default netns.

To simplify wireshark remote capturing process users can create a tiny bash script that will save some typing:

```bash
#!/bin/sh
# call this script as `bash script_name.sh <container-name> <interface-name>`
ssh <containerlab_address> "ip netns exec $1 tcpdump -U -nni $2 -w -" | wireshark -k -i -

```