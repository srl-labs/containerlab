---
comments: true
---

# Packet capture & Wireshark

Every lab emulation software must provide its users with the packet capturing abilities. Looking at the frames as they traverse the network links is not only educational, but also helps to troubleshoot the issues that might arise during the lab development.

<p align=center>
<img src="https://gitlab.com/rdodin/pics/-/wikis/uploads/a7bffdae0393c9de41545c627f9b9f30/wsh.png" width="40%"></img>
</p>

Containerlab offers a simple way to capture the packets from any interface of any node in the lab. This article will explain how to do that.

///tip

If you are looking for a free Web UI for packet capture with Wireshark, check out our [Edgeshark integration](#edgeshark-integration).
///

Consider the following lab topology which highlights the typical points of packet capture.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:13,&quot;zoom&quot;:2,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

Since containerlab leverages Linux network devices, users can use any tool to sniff packets. Let's see how one can use either or a combination of the following well-known packet capturing tools: `tcpdump`, `tshark` and `wireshark`.

## Packet capture, namespaces and interfaces

To capture the packets from a given interface requires having that interface's name and its network namespace (netns). And that's it.

The diagram below shows the two nodes connected with a single link and how network namespaces are used to isolate one node's networking stack from another. Looking at the diagram, it is clear which interface belongs to which network namespace.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlceos01.drawio&quot;}"></div>

When a lab like the one above is deployed, containerlab creates the following containers:

- `clab-quickstart-srl`
- `clab-quickstart-ceos`

And the namespace names would be named accordingly to the container names, namely `clab-quickstart-srl` and `cla-quickstart-ceos`.

## Capture modes

Now when it is clear which netns names corresponds to which container and which interfaces are available inside the given lab node, it's easy to start capturing traffic. But how to do that?

There are two most common ways of capturing the traffic:

- local capture: when the capture is started from the containerlab host itself.
- remote capture: when the capture is started from a remote machine that connects via SSH to the containerlab host and starts the capture.

In both cases, the capturing software (`tcpdump` or `tshark`) needs to be available on the containerlab host.

### local capture

Local capture assumes the capture is initiated from the containerlab host. For instance, to capture from the `e1-1` interface of the `clab-quickstart-srl` node use:

```bash
ip netns exec clab-quickstart-srl tcpdump -nni e1-1
```

In this example we first entered the namespace where the target interface is located using `ip netns exec` command and then started the capture with `tcpdump` providing the interface name to it.

The downside of local capture is that typically containerlab hosts run in a headless (no UI) mode and thus the visibility of the captured traffic is limited to the console output. This is where `tshark` might come in in handy by providing more readable output. Still, the lack of Wireshark UI is a downside, therefore it is our recommendation for you to get familiar with the remote capture method.

### remote capture

The limitations of the local capture are lifted when the remote capture is used. In the remote capture model you initiate the packet capture from your descktop/laptop and the traffic is sent to your machine where it can be displayed in the Wireshark UI.

Before we start mixing in the Wireshark, lets see how the remote capture is initiated:

```bash
ssh $containerlab_host_address \
    "ip netns exec clab-quickstart-srl tcpdump -nni e1-1"
```

Assuming we ran the above command from our laptop, we rely on `$containerlab_host_address` being reachable from our laptop. We use `ssh` to connect to the remote containerlab host and execute the same command we did in the local capture.  

But simply seeing the tcpdump output on your laptop's terminal doesn't offer much difference to the local capture.

The true power the remote capture has is in being able to use the Wireshark installed on your machine to display the captured traffic. To do that we need to pipe the output of the `tcpdump` command to the `wireshark` command. This is done by adding the `-w -` option to the `tcpdump` command which tells it to write the captured traffic to the standard output. The output is then piped to the `wireshark` command which is invoked with the `-k -i -` options. The `-k` option tells wireshark to start capturing immediately and the `-i -` option tells it to read the input from the standard input.

```bash
ssh $containerlab_host_address \
    "ip netns exec $lab_node_name tcpdump -U -nni $if_name -w -" | \
    wireshark -k -i -
```

This will start the capture from a given interface and redirect the received flow to the Wireshark!

<video width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/6a0b8fb25d46b3764e8b2ce4667c07f7/wireshark.mp4" type="video/mp4">
</video>

///note | Windows users
Windows users should use WSL and invoke the command similar to the following:

```bash
ssh $containerlab_host_address \
    "ip netns exec $lab_node_name tcpdump -U -nni $if_name -w -" | \
    /mnt/c/Program\ Files/Wireshark/wireshark.exe -k -i -
```

///

## Capture script

Since capturing is a so popular it makes sense to create a tiny helper script that will simplify the process of capturing from a given interface of a given node. The script presented below hides all the irrelevant details and makes sniffing a breeze. Let's see how it works:

```bash title="pcap.sh"
#!/bin/sh
# call this script as
# pcap.sh <containerlab-host> <container-name> <interface-name>
# example: pcap.sh clab-vm srl e1-1

# to support multiple interfaces, pass them as comma separated list
# split $3 separate by comma as -i <interface1> -i <interface2> -i <interface3>
IFS=',' read -ra ADDR <<< "$3"
IFACES=""
for i in "${ADDR[@]}"; do
    IFACES+=" -i $i"
done

ssh $1 "sudo ip netns exec $2 tshark -l ${IFACES} -w -" | \
    /Applications/Wireshark.app/Contents/MacOS/Wireshark -k -i -
```

If you put this script somewhere in your `$PATH` you can invoke it as follows:

```bash
pcap.sh clab-vm srl e1-1
```

where

- `clab-vm` is the containerlab host address that is reachable from your machine
- `srl` is the container name of the node that has the interface you want to capture from
- `e1-1` is the interface name you want to capture from

The script uses the `tshark` CLI tool instead of `tcpdump` to be able to capture from multiple interfaces at once[^1]. This is achieved by splitting the interface names by comma and passing them to the `tshark` command as `-i <interface1> -i <interface2> -i <interface3>`.

Here is a short demo of how the script works and how to use it based on the lab topology with linux nodes and SR Linux NOS:

<div class="iframe-container">
<iframe width="100%" src="https://www.youtube.com/embed/qojiQ38troc" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
</div>

///note
The script uses the Mac OS version of the Wireshark. If you are on Linux, you can simply replace the last line with `wireshark -k -i -`.
///

## Edgeshark integration

The [capture script](#capture-script) already makes it easy to dump packets off of an interface and piping it to a Wireshark UI. Is there anything that can make it even easier?

How about a Web UI that displays every interface of every container and can start a wireshark session by a click of a button? Let us introduce you to the [Edgeshark][edgeshark-docs].

<video autoplay loop width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/fbffd718f7f64a7920f75b6363b98002/2024-02-07_20-07-47.mp4" type="video/mp4">
</video>

///admonition
    type: quote
[Edgeshark][edgeshark-docs] visualizes the communication of containers and thus helps in diagnosing it, both in-between containers as well as with the "outside world". It can be deployed to Linux stand-alone container hosts, including KinD deployments. Edgeshark also supports capturing container traffic using Wireshark.
///

Yep, you got it right, edgeshark is a Web UI for Wireshark[^2] that is capable of capturing traffic from any interface of any container (and physical interface) in your lab. Moreover, it plugs into containerlab natively, and is free and open-source.

This diagram shows a typical integration of edgeshark with containerlab:

<div class='mxgraph' style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;' data-mxgraph='{"page":0,"zoom":2,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank","url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/pcap.drawio"}'></div>

From a user's perspective, the integration is as simple as running the following command on the containerlab host:

/// tab | Install

```bash
curl -sL \
https://github.com/siemens/edgeshark/raw/main/deployments/wget/docker-compose.yaml \
| DOCKER_DEFAULT_PLATFORM= docker compose -f - up -d
```

///
/// tab | Uninstall

```bash
curl -sL \
https://github.com/siemens/edgeshark/raw/main/deployments/wget/docker-compose.yaml \
| DOCKER_DEFAULT_PLATFORM= docker compose -f - down
```

///

This will deploy the edgeshark containers and expose the Web UI on the containerlab host's port 5001. If you have a network reachability to the containerlab host, you can open the Web UI (`https://<containerlab-host-address>:5001`) in your browser and see the Edgeshark UI.

/// details | SSH port forwarding in case you don't have direct reachability
If you don't have direct reachability, you can use SSH port forwarding to access the Web UI:

```bash title="ssh port forwarding"
ssh -L 5001:localhost:5001 $containerlab_host
```

Then open your browser and navigate to `http://localhost:5001` to see the Edgeshark UI.
///

Check out the Edgeshark & Containerlab integration in action:

<div class="iframe-container">
<iframe width="100%" src="https://www.youtube.com/embed/iY90a_Gn5W0" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
</div>

### Wireshark and system configuration

There is a small price one needs to pay to make integrate Edgeshark with Wireshark, it consists of two steps:

1. Configuring the system to handle the `packetflix://` URL schema and open it with the Wireshark.
2. Installing the external capture plugin for Wireshark.

Luckily, you only need to do it once and it will work for all the future captures on any system you install EdgeShark on.

[Edgeshark documentation](https://edgeshark.siemens.io/#/getting-started?id=optional-capture-plugin) provides a detailed guide on how to perform these two steps for Windows and Linux systems.

There was a tiny gap in MacOS support, but we contributed the necessary piece[^3] and here is how you configure your MacOS system to work with Edgeshark:

1. Download the zip file with the AppleScript that enables the `packetflix://` URL schema handling, unarchive it and move the EdgeShark-handler script to the Applications folder. Here is the script that does all that, run it from your MacOS:

    ```bash
    mkdir -p /tmp/pflix-handler && cd /tmp/pflix-handler && \
    rm -rf packetflix-handler.zip packetflix-handler.app __MACOSX && \
    curl -sLO https://github.com/srl-labs/containerlab/files/14278951/packetflix-handler.zip && \
    unzip packetflix-handler.zip && \
    sudo mv packetflix-handler.app /Applications
    ```

2. Download the [external capture plugin](https://github.com/siemens/cshargextcap/releases/latest) for Darwin OS and your architecture, unarchive and copy it with Finder[^4] to the `/Applications/Wireshark.app/Contents/MacOS/extcap/` directory.

With these steps done[^5], you should be able to click on the "fin" icon next to the interface name and see the Wireshark UI opening up and starting the capture.

## VS Code extension

The [Containerlab's VS Code extension](../manual/vsc-extension.md#integrated-wireshark) takes the packet capture experience to the next level by offering the convenience of Edgeshark without requiring you to install the capture plugins or configure the system to handle the `packetflix://` URL schema. The extension takes care of all the setup process and packages the components in the containers that will be run by the extension, all you need to do is to select the interface to capture from.

## Examples

Lets take the first diagram of this article and see which commands are used to sniff from the highlighted interfaces.

In the examples below the wireshark will be used as a sniffing tool and the following naming simplifications and conventions used:

- `$clab_host` - address of the containerlab host
- `clab-pcap-srl`, `clab-pcap-ceos`, `clab-pcap-linux` - container names of the SRL, cEOS and Linux nodes accordingly.

///tab | SR Linux [1], [4]
SR Linux linecard interfaces are named as `e<linecard_num>-<port_num>` which translates to `ethernet-<linecard_num>/<port_num>` name inside the NOS itself.  
So to capture from `ethernet-1/1` interface the following command should be used:

```bash
ssh $clab_host \
    "ip netns exec $clab-pcap-srl tcpdump -U -nni e1-1 -w -" | \
    wireshark -k -i -
```

The management interface on the SR Linux container is named `mgmt0`, so the relevant command will look like:

```bash
ssh $clab_host \
    "ip netns exec $clab-pcap-srl tcpdump -U -nni mgmt0 -w -" | \
    wireshark -k -i -
```

///
///tab | cEOS [2]
Similarly to SR Linux example, to capture the data interface of cEOS is no different. Just pick the right interface:

```bash
ssh $clab_host \
    "ip netns exec $clab-pcap-ceos tcpdump -U -nni eth1 -w -" | \
    wireshark -k -i -
```

///
///tab | Linux container [3]
A bare linux container is no different, its interfaces are named `ethX` where `eth0` is the interface connected to the containerlab management network.  
So to capture from the first data link we will use `eth1` interface:

```bash
ssh $clab_host \
    "ip netns exec $clab-pcap-linux tcpdump -U -nni eth1 -w -" | \
    wireshark -k -i -
```

///
/// tab | management bridge [5]
It is also possible to listen for all management traffic that traverses the containerlab's management network. To do that you firstly need to [find out the name of the linux bridge](network.md#connection-details) and then capture from it:

```bash
ssh $clab_host "tcpdump -U -nni brXXXXXX -w -" | wireshark -k -i -
```

Note that in this case you do not need to drill into the network namespace, since management bridge is in the default netns.
///

## Useful commands

To list available network namespaces:

```bash
ip netns list
```

To list the interfaces (links) of a given container leverage the `ip` utility:

```bash
# where $netns_name is the container name of a node
ip netns exec $netns_name ip link
```

[edgeshark-docs]: https://edgeshark.siemens.io/#/

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

[^1]: Install `tshark` if you don't have it, or edit the script to use `tcpdump` instead.
[^2]: It is more than just a UI for Wireshark, but in the context of pcap capture we focus on this feature solely.
[^3]: https://github.com/siemens/cshargextcap/issues/14#issuecomment-1932267889
[^4]: If you granted your Terminal app full permission to the disk you could use `cp` instead of a Finder, but if you see "operation not permitted" error, then try using Finder when copying the file.
[^5]: You may be asked to allow running the application downloaded from Internet as per MacOS security policy.
