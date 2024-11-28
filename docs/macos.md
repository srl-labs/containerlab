---
comments: true
hide:
- navigation
---

# Containerlab on macOS

/// details | Summary for the impatient
    type: subtle-note

1. Install [OrbStack](https://orbstack.dev)[^1] on your macOS
2. Create an **arm64** Linux VM using OrbStack or alternatives
3. Install containerlab in the VM using the usual [installation instructions](install.md)
4. Check what images can/should work on ARM64
5. Deploy your lab. You can see the demo of this workflow in [this YT video][yt-demo].

[yt-demo]: https://www.youtube.com/watch?v=_BTa-CiTpvI&t=1573s

If you run an Intel mac, you can still use OrbStack to deploy a VM, but you will not need to worry about ARM64 images, as your processor runs x86_64 natively.
///

For quite some time, we have been saying that containerlab and macOS is a challenging mix. This statement has been echoed through multiple workshops/demos and was based on the following reasons:

1. **ARM64 Network OS images**: With the shift to ARM64 architecture made by Apple (and Microsoft[^2]), we found ourselves in a situation where 99% of existing network OSes were not compiled for ARM64 architecture. This meant that containerlab users would have to rely on x86_64 emulation via Rosetta or QEMU, which imposes a significant performance penalty, often making the emulation unusable for practical purposes.
2. **Docker on macOS**: Since containerlab is reliant on Docker for container orchestrations, it needs Docker to be natively installed on the host.  
    On macOS, Docker is always provided as a Linux/arm64 VM that runs the docker daemon, with docker-cli running on macOS natively. You can imagine, that dealing with a VM that runs a network topology poses some UX challenges, like getting access to the exposed ports or dealing with files on macOS that needs to be accessible to the Docker VM.  
3. **Linux on macOS?** It is not only Docker that containerlab is based on. We leverage some Linux kernel APIs (like netlink) either directly or via Docker to be available to setup links, namespaces, bind-mounts, etc.  
    Naturally, Darwin (macOS kernel) is not Linux, and while it is POSIX compliant, it is not a drop-in replacement for Linux. This means that some of the Linux-specific features that containerlab relies on are simply not present on macOS.

Looking at the above challenges one might think that containerlab on macOS is a lost cause. However, recently things have started to take a good course, and we are happy to say that for certain labs Containerlab on macOS might be even a better (!) choice overall.

As a long time macOS user, Roman recorded an in-depth video demonstrating how to run containerlab topologies on macOS using the tools of his choice. You can watch the video below or jump to the text version of the guide below.

-{{youtube(url='https://www.youtube.com/embed/_BTa-CiTpvI')}}-

## Network OS Images

The first thing one needs to understand that if you run macOS on ARM chipset (M1+), then you should use ARM64 network OS images whenever possible. This will give you the best performance and compatibility.

With Rosetta virtualisation it is possible to run x86_64 images on ARM64, but this comes with the performance penalty that might even make nodes not work at all.

/// admonition | VM-based images
    type: warning
VM-based images built with [hellt/vrnetlab](manual/vrnetlab.md) require nested virtualization support, which is only available on M3+ chip with macOS version 15 and above.

If you happen to satisfy these requirements, please let us know in the comments which images you were able to run on your M3+ Mac.
///

### Native ARM64 Network OS and application images

Finally :pray: some good news on this front, as vendors started to release or at least announce ARM64 versions of their network OSes.  
**Nokia** first [released](https://www.linkedin.com/posts/rdodin_oops-we-did-it-again-three-years-ago-activity-7234176896018632704-Ywk-/) the preview version of their freely distributed [SR Linux for ARM64](manual/kinds/srl.md#getting-sr-linux-image), and **Arista** announced the imminent cEOS availability sometime in 2024.

You can also get [**FRR**](https://quay.io/repository/frrouting/frr?tab=tags) container for ARM64 architecture from their container registry.

Yes, SR Linux, cEOS, FRR do not cover all the network OSes out there, but it is a good start, and we hope that more vendors will follow the trend.

The good news is that almost all of the popular applications that we see being used in containerlabs are **already** built for ARM. Your streaming telemetry stack (gnmic, prometheus/influx, grafana), regular client-emulating endpoints such as Alpine or a collection of network related tools in the network-multitool image had already been supporting ARM architecture. You can leverage the sheer ecosystem multi-arch applications that are available in the public registries.

### Running under Rosetta

If the image you're looking for is not available in ARM64, you can still try running the AMD64 version of the image under Rosetta emulation. Rosetta is a macOS virtualisation layer that allows you running x86_64 code on ARM64 architecture.

It has been known to work for the following images:

- [Arista cEOS x64](manual/kinds/ceos.md)
- [Cisco IOL](manual/kinds/cisco_iol.md)

## Docker on macOS

Ever since macOS switched to ARM architecture for their processors, people in a "containers camp" have been busy making sure that Docker works well on macOS's new architecture.

### How Docker runs on Macs

But before we start talking about Docker on ARM Macs, let's remind ourselves how Docker works on macOS with Intel processors.

-{{ diagram(url='srl-labs/containerlab/diagrams/macos-arm.drawio', title='Docker on Intel Macs', page=3) }}-

At the heart of any product or project that enables the Docker engine on Mac[^3] is a Linux VM that hosts the Docker daemon, aka "the engine". This VM is created and managed by the application that sets up Docker on your desktop OS.  
The Linux VM is a mandatory piece because the whole container ecosystem is built around Linux kernel features and APIs. Therefore, running Docker on any host with an operating system other than Linux requires a Linux VM.

As shown above, on Intel Macs, the macOS runs Darwin kernel on top of an AMD64 (aka x86_64) architecture, and consequently, the Docker VM runs the same architecture. The architecture of the Docker VM is the same as the host architecture allowing for a performant virtualization, since no processor emulation is needed.

Now let's see how things change when we switch to ARM Macs:

-{{ diagram(url='srl-labs/containerlab/diagrams/macos-arm.drawio', title='Docker on ARM Macs', page=2) }}-

The diagram looks 99% the same as for the Intel Macs, the only difference being the architecture that macOS runs on and consequently the architecture of the Docker VM.  
Now we run ARM64 architecture on the host, and the Docker VM is also ARM64.

/// details | Native vs Emulation

If Docker runs exactly the same on ARM Macs as it does on Intel Macs, then why is it suddenly a problem to run containerlab on ARM Macs?

Well, it all comes down to the requirement of having ARM64 network OS images that we discussed earlier. Now when your Docker VM runs Linux/ARM64, you can run natively only ARM64-native software in it, and we, as a network community, are not having a lot of ARM64-native network OSes. It is getting better, but we are not there yet to claim 100% parity with the x86_64 world.

You should strive to run the native images as much as possible, as it gives you the best performance and compatibility. But how do you tell if the image is ARM64-native or not?  
A lot of applications that you might want to run in your containerlab topologies are already ARM64-native and often available as a multi-arch image.

When running the following `docker image inspect` command, you can grep the `Architecture` field to see if the image is ARM64-native:

```bash
docker image inspect ghcr.io/nokia/srlinux:24.10.1 -f '{{.Architecture}}'
arm64
```

Running the same command for an image that is not ARM64-native will return `amd64`:

```bash
docker image inspect goatatwork/snmpwalk -f '{{.Architecture}}'
amd64
```

Still, it will be possible to run the `snmpwalk` container, thanks to Rosetta emulation.
///

### Software

There are many software solutions that deliver Docker on macOS, both for Intel and ARM Macs.

- :star: [OrbStack](https://orbstack.dev/) - a great UX and performance. A choice of many and is recommended by Containerlab maintainer. Free for personal use.
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) - the original and the most popular Docker on macOS.
- [Rancher Desktop](https://rancherdesktop.io/) - another popular software.
- [CoLima](https://github.com/abiosoft/colima) - a lightweight, CLI-based VM solution.

The way most users use Containerlab on macOS, though, not directly leveraging Docker that is provided by one of the above solutions. Instead, it might be easier to spin up a VM, powered by the above-mentioned software products, and install Containerlab natively inside this arm64/Linux VM.  
You can see this workflow demonstration in this [YT video][yt-demo].

## Devcontainer

Another convenient option to run containerlab on ARM/Intel Macs and Windows is to use the [Devcontainer](https://docs.github.com/en/codespaces/setting-up-your-project-for-codespaces/adding-a-dev-container-configuration/introduction-to-dev-containers) feature that works great with VS Code and many other IDE's.

A Devcontainer is a specification for a container that is used to create a development environment. By creating the `devcontainer.json` file, you define the development environment for your project. Containerlab project maintains a set of pre-built multi-arch devcontainer images that you can use to run containerlabs.  
It was initially created to power [containerlab in codespaces](manual/codespaces.md), but it is a perfect fit for running containerlab on a **wide range of OSes** such as macOS and Windows.

The devcontainer can also be used by the developers to develop containerlab itself.

/// note
Starting with **Containerlab v0.60.0**, you can use the devcontainer with ARM64 macOS to run containerlabs.
///

To start using the devcontainer, you have to create a `devcontainer.json` file in your project directory where you have your containerlab topology. If you're using Containerlab the right way, your labs are neatly stored in a git repo, the `devcontainer.json` file will be part of the repo.

/// note | Devcontainer flavors
Containerlab maintains two types of devcontainers configurations:

1. Docker In Docker (DIND):  
    tagged as `ghcr.io/srl-labs/containerlab/clab-devcontainer-dind:<version>`, where `<version>` is the containerlab version.

    This
///

Note, the labs that we publish with Codespaces support already have the `devcontainer.json` file, in that case you don't even need to create it manually.

If you create the `devcontainer.json` file manually, you won't need to be smart about the content of the file, all you have to specify is the containerlab version you want to run. Here is an example of the `devcontainer.json` file:

```json title="<code>./devcontainer/devcontainer.json</code>"
{
    "image": "ghcr.io/srl-labs/containerlab/clab-devcontainer:0.60.0" //(1)!
}
```

1. devcontainer versions match containerlab versions

With the devcontainer file in place, when you open a repo in VS Code, you will be prompted to reopen the workspace in the devcontainer.

![img1](https://gitlab.com/rdodin/pics/-/wikis/uploads/ee918d1d5d85d83f45ced031c5fa999d/image.png)

Clicking on this button will open the workspace in the devcontainer; you will see the usual VS Code window, but now the workspace will have containerlab installed with a separate docker instance running inside the spawned container. This means that your devcontainer works in isolation with the rest of your system.

Open a terminal in the VS Code and run the topology by typing the familiar `sudo clab dep` command to deploy the lab. That's it!

## Alternative options

### UTM

If OrbStack for some reason can not be used in your environment, you can use [UTM](https://mac.getutm.app/) - a free[^4] and open-source graphical virtual machine manager that provides a simple interface for creating, managing, and running virtual machines with qemu.

When you have UTM installed, you can download a pre-built Debian 12 UTM image built by the Containerlab team using the following command[^5]:

```bash
sudo docker run --rm -v $(pwd):/workspace ghcr.io/oras-project/oras:v1.1.0 pull \
    ghcr.io/srl-labs/containerlab/clab-utm-box:0.1.0
```

By running this command you will download the `clab_debian12.utm` file which is a UTM image with `containerlab`, `docker-ce` and `gh` tools pre-installed[^6].

Open the downloaded image with UTM **File -> Open -> select .utm file** and start the VM.

Once the VM is started, you can log in using `debian:debian` credentials. Run `ip -4 addr` in the terminal to find out which IP got assigned to this VM.  
Now you can use this IP for your Mac terminal to connect to the VM via SSH[^7].

When logged in, you can upgrade the containerlab to the latest version with:

```bash
sudo clab version upgrade
```

and start downloading the labs you want to run.

[^1]: Or any other application that enables Docker on macOS. OrbStack is just a great choice that is used by many.
[^2]: With Microsoft Surface laptop released with ARM64 architecture
[^3]: The same principles apply to Docker Desktop on Windows
[^4]: There are two options to install UTM: via [downloadable dmg](https://github.com/utmapp/UTM/releases/latest/download/UTM.dmg) file (free) or App Store (paid). The App Store version is exactly the same, it is just a way to support the project.
[^5]: This command requires docker to be installed on your macOS. You can use Docker Desktop, Rancher or [colima](https://github.com/abiosoft/colima) to run docker on your macOS.
[^6]: If you want to install these tools on an existing Debian machine, you can run `wget -qO- containerlab.dev/setup-debian | bash -s -- all` command.
[^7]: The UTM image has a pre-installed ssh key for the `debian` user. You can download the shared private key from [here](https://github.com/srl-labs/clabernetes/blob/main/launcher/assets/default_id_rsa).

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
