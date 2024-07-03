---
search:
  boost: 4
---
# SONiC (VM)

[SONiC](https://sonic-net.github.io/SONiC/) Network OS is distributed in two formats suitable for testing with containerlab

1. Containerized SONiC (`sonic-vs` kind)
2. Virtual Machine SONiC (`sonic-vm` kind; the topic of this document)

This document covers the VM flavor of the upstream SONiC that is identified with `sonic-vm` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `sonic-vm` node.

The VM-based image of SONiC is built with the [`hellt/vrnetlab`](https://github.com/hellt/vrnetlab/tree/master/sonic) project.

## Getting Sonic images

Getting SONiC images is possible via two resources:

1. [Sonic.software](https://sonic.software/) -- an unofficial repo with SONiC images (may be down sometimes, uses Azure pipeline as a source)
2. [Azure pipeline](https://sonic-build.azurewebsites.net/ui/sonic/pipelines) -- an official source of SONiC images (may also be down eventually), and finding the right one there is a pita.
3. [Another pipeline view](https://sonic-net.github.io/SONiC/sonic_latest_images.html) -- may not contain recent pipelines.

When https://sonic.software is down, you can follow the following procedure to find the SONiC image in the Azure pipeline artifacts maze:

1. Go to the piplines list: https://sonic-build.azurewebsites.net/ui/sonic/pipelines
2. Scroll all the way to the bottom where `vs` platform is listed
3. Pick a branch name that you want to use (e.g. `202405`) and click on the "Build History".
4. On the build history page choose the latest build that has succeeded (check the Result column) and click on the "Artifacts" link
5. In the new window, you will see a list with a single artifact, click on it
6. One more long scroll down until you see `target/sonic-vs.img.gz` name (or Ctrl+F for it), click on it to start the download or copy the download link.
7. Here you go, you managed to download a SONiC image from a mysteriosly named branch for a build that probably means nothing to you. This Sonic experience for ya...

/// details | How to download SONiC image from Azure pipeline (video)
<video width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/054c60a0c8d685f826297c115470221b/sonic-dl.mp4" type="video/mp4">
</video>
///

## Managing sonic-vm nodes

SONiC node launched with containerlab takes approximately 1 minute to boot up and can be managed via the following interfaces:

/// note
The default login credentials for the SONiC VM are `admin:admin`
///

/// tab | SSH
To open a linux shell simply type in

```bash
ssh <node-name>
```

You will enter the bash shell of the VM:

```
❯ ssh clab-sonic-sonic 
Warning: Permanently added 'clab-sonic-sonic' (RSA) to the list of known hosts.
Debian GNU/Linux 12 \n \l

admin@clab-sonic-sonic's password: 
Linux sonic 6.1.0-11-2-amd64 #1 SMP PREEMPT_DYNAMIC Debian 6.1.38-4 (2023-08-08) x86_64
You are on
  ____   ___  _   _ _  ____
 / ___| / _ \| \ | (_)/ ___|
 \___ \| | | |  \| | | |
  ___) | |_| | |\  | | |___
 |____/ \___/|_| \_|_|\____|

-- Software for Open Networking in the Cloud --

Unauthorized access and/or use are prohibited.
All access and/or use are subject to monitoring.

Help:    https://sonic-net.github.io/SONiC/

Last login: Wed Jul  3 09:45:35 2024
admin@sonic:~$
```

From within the Linux shell users can perform system configuration using linux utilities, or connect to the SONiC CLI using `vtysh` command.

```
admin@sonic:~$ vtysh

Hello, this is FRRouting (version 8.5.4).
Copyright 1996-2005 Kunihiro Ishiguro, et al.

sonic#
```

///
/// tab | Telnet
to connect to sonic-vm CLI via telnet

```bash
telnet <container-name/id> 5000
```

///

## Interfaces mapping

sonic-vm container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data (front-panel port) interface that is mapped to Ethernet0 port
* `eth2` - second data interface that is mapped to Ethernet4 port. Any new port will result in a "previous interface + 4" (Ethernet4) mapping.

When containerlab launches sonic-vs node, it will assign IPv4/6 address to the `eth0` interface. Data interface `eth1` mapped to `Ethernet0` port and needs to be configured with IP addressing manually.

## Features and Options

### Startup configuration

VM-based SONiC supports the [`startup-config`](../nodes.md#startup-config) feature. The startup configuration file is a JSON file that is available in the VM's filesystem by the `/etc/sonic/config_db.json` path.

Extracting the config from a running node is possible with `containerlab save` command. The config will be available in the lab directory.
