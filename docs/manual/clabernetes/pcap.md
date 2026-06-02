# Packet capture in Clabernetes

It is quite interesting to see how Clabernetes uses different datapath stitching tricks to connect lab nodes running in different containers. Sometimes looking at the packets exchanged between the nodes can help to understand the inner workings of the setup and often comes in handy when troubleshooting.

Capturing packets in Clabernetes is similar to capturing packets in Containerlab, with just one more indirection level added. Check the basics of [packet capture in Containerlab](../wireshark.md) to get started, because we will use the same technique here.

## Capture script

The most straightforward way to capture packets in Clabernetes is to leverage the capture script similar to the one we did for Containerlab, but instead of using `ip netns exec` we will use `kubectl exec` to run the packet capture in the container and piping the output to Wireshark.

Below you will find two script variants, one for a case when `kubectl` runs on the same machine where Wireshark is installer, and the second one when the `kubectl` runs on a remote machine.

/// note
The examples below are given for the MacOs, for Windows users running WSL the path to the Wireshark will be `/mnt/c/Program\ Files/Wireshark/wireshark.exe` and Linux users will figure it out without hints :wink:
///

/// tab | local kubectl
Since the `kubectl` is installed locally, we can straight away use `kubectl exec` to connect to the pod.
The script below is used like:

```bash
bash c9spcap.sh <k8s-namespace> <pod name> <interface name>
```

```bash title="c9spcap.sh"
#!/bin/sh

kubectl exec -n $1 -it $2 -- tcpdump -U -nni $3 -w - | \
/Applications/Wireshark.app/Contents/MacOS/Wireshark -k -i -
```

///
/// tab | remote kubectl
Since the `kubectl` is installed remotely, we need to use `ssh` to connect to the remote machine first.
The script below is used like:

```bash
bash c9spcap.sh <host-with-kubectl> <k8s-namespace> <pod name> <interface name>
```

```bash title="c9spcap.sh"
#!/bin/sh

ssh $1 "kubectl exec -n $2 -it $3 -- tcpdump -U -nni $4 -w -" | \
/Applications/Wireshark.app/Contents/MacOS/Wireshark -k -i -
```

///

It is a smart idea to save the script in a directory that is in your `PATH` so that you can run it from anywhere anytime you need to capture some packets.
