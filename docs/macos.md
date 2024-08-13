---
comments: true
hide:
- navigation
---

# Containerlab on macOS

For quite some time, we have been saying that containerlab and macOS is a challenging mix. This statement has been echoed through multiple workshops, presentations, and demos was based on the following reasons:

1. **ARM64 Network OS images**: With the shift to ARM64 architecture made by Apple (and Microsoft[^1]), we found ourselves in a situation where 99% of existing network OSes were not compiled for ARM64 architecture. This meant that containerlab users would have to rely on x86_64 emulation via Rosetta or QEMU, which imposes a significant performance penalty, often making the emulation unusable for practical purposes.
2. **Docker on macOS**: Since containerlab is reliant on Docker for container orchestrations, it needs Docker to be natively installed on the host.  
    On macOS, Docker is always provided as a Linux/arm64 VM that runs the docker daemon, with docker-cli running on macOS natively. You can imagine, that dealing with a VM that runs a network topology poses some UX challenges, like getting access to the exposed ports or dealing with files on macOS that needs to be accessible to the Docker VM.  
3. **Linux on macOS?** It is not only Docker that containerlab is based on. We leverage some Linux kernel APIs (like netlink) either directly or via Docker to be available to setup links, namespaces, bind-mounts, etc.  
    Naturally, Darwin (macOS kernel) is not Linux, and while it is POSIX compliant, it is not a drop-in replacement for Linux. This means that some of the Linux-specific features that containerlab relies on are simply not present on macOS.

Looking at the above challenges one might think that containerlab on macOS is a lost cause. However, recently things have started to change quite rapidly, and we are happy to say that for certain labs Containerlab on macOS might be a better choice overall.

As a long time macOS user, Roman recorded an in-depth video demonstrating how to run containerlab topologies on macOS using the tools of his choice. You can watch the video below or jump to the text version of the guide below.

> VIDEO HERE

## ARM64 Network OS and application images

Finally :pray: some good news on this front, as vendors started to release or at least announce ARM64 versions of their network OSes.  
**Nokia** first released the preview version of their freely distributed [SR Linux for ARM64](manual/kinds/srl.md#getting-sr-linux-image), and **Arista** announced the imminent cEOS availability sometime in 2024.

You can also get [**FRR**](https://quay.io/repository/frrouting/frr?tab=tags) container for ARM64 architecture from their container registry.

/// details | What about VM-based images?
Unfortunately, we don't know if we will see ARM64 versions of the VM-based network OSes from the likes of Cisco and Juniper.

You may still try running some of them using Rosetta emulation, but if you manage to get the VM booted, it might be very slow and not usable for practical purposes. Still worth a try though.
///

Yes, SR Linux, cEOS, FRR do not cover all the network OSes out there, but it is a good start, and we hope that more vendors will follow the trend.

The good news is that almost all of the popular applications that we see being used in containerlabs are **already** built for ARM. Your streaming telemetry stack (gnmic, prometheus/influx, grafana), regular client-emulating endpoints such as Alpine or a collection of network related tools in the network-multitool image had already been supporting ARM architecture. You can leverage the sheer ecosystem multi-arch applications that are available in the public registries.

## Docker on macOS

Ever since macOS switched to ARM architecture for their processors, people in "containers camp" have been busy making sure that Docker works well on macOS's new architecture.

### How Docker runs on Macs

But before we start talking about Docker on ARM Macs, let's remind ourselves how Docker works on macOS with Intel processors.

<figure markdown>
  <div class='mxgraph' style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;' data-mxgraph='{"page":3,"zoom":2,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank","url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/macos-arm.drawio"}'></div>
  <figcaption>Docker on Intel Macs</figcaption>
</figure>

At the heart of any product or project that enables the Docker engine on Mac[^2] is a Linux VM that hosts the Docker daemon, aka "the engine". This VM is created and managed by the application that sets up Docker on your desktop OS.  
The Linux VM is a mandatory piece because the whole container ecosystem is built around Linux kernel features and APIs. Therefore, running Docker on any host with an operating system other than Linux requires a Linux VM.

As shown above, on Intel Macs, the macOS runs Darwin kernel on top of an AMD64 (aka a86_64) architecture, and consequently, the Docker VM runs the same architecture. The architecture of the Docker VM is the same as the host architecture allowing for a performant virtualization, since no processor emulation is needed.

Now let's see how things change when we switch to ARM Macs:

<figure markdown>
  <div class='mxgraph' style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;' data-mxgraph='{"page":2,"zoom":2,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank","url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/macos-arm.drawio"}'></div>
  <figcaption>Docker on ARM Macs</figcaption>
</figure>

The diagram looks 99% the same as for the Intel Macs, the only difference being the architecture that macOS runs on and consequently the architecture of the Docker VM.  
Now we run ARM64 architecture on the host, and the Docker VM is also ARM64.

### Native vs Emulation

If Docker runs exactly the same on ARM Macs as it does on Intel Macs, then why is it suddenly a problem to run containerlab on ARM Macs?x`

Well, it all comes down to the requirement of having ARM64 network OS images that we dicsussed earlier. Now when your Docker VM runs Linux/ARM64, you can run natively only ARM64-native software in it, and we, as a network community, are not having a lot of ARM64-native network OSes. It is getting better, but we are not there yet to claim 100% parity with the x86_64 world.

You should strive to run the native images as much as possible, as it gives you the best performance and compatibility. But how do you tell if the image is ARM64-native or not?  
A lot of applications that you might want to run in your containerlab topologies are already ARM64-native and often available as a multi-arch image.

[^1]: With Microsoft Surface laptop released with ARM64 architecture
[^2]: The same principles apply to Docker Desktop on Windows

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
