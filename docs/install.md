---
hide:
  - navigation
---
Containerlab is distributed as a Linux deb/rpm package and can be installed on any Debian- or RHEL-like distributive in a matter of a few seconds.

## Pre-requisites

The following requirements must be satisfied to let containerlab tool run successfully:

* A user should have `sudo` privileges to run containerlab.
* A Linux server/VM[^2] and [Docker](https://docs.docker.com/engine/install/) installed.
* Load container images (e.g. Nokia SR Linux, Arista cEOS) that are not downloadable from a container registry. Containerlab will try to pull images at runtime if they do not exist locally.

## Quick setup

The easiest way to get started with containerlab is to use the [quick setup script](https://github.com/srl-labs/containerlab/blob/main/utils/quick-setup.sh) that installs all of the following components in one go (or allows to install them separately):

* docker (docker-ce), docker compose
* Containerlab (using the package repository)
* [`gh` CLI tool](https://cli.github.com/)

The script officially supports the following OSes:

* Ubuntu 20.04, 22.04, 23.10
* Debian 11, 12
* Red Hat Enterprise Linux 9
* CentOS Stream 9
* Fedora Server 40 (should work on other variants of Fedora)
* Rocky Linux 9.3, 8.8 (should work on any 9.x and 8.x release)

To install all components at once, run the following command on any of the supported OSes:

```bash
curl -sL https://containerlab.dev/setup | sudo bash -s "all"
```

/// note
To complete installation please execute `newgrp docker` or logout and log back in.
///

To install an individual component, specify the function name as an argument to the script. For example, to install only `docker`:

```bash
curl -sL https://containerlab.dev/setup | sudo bash -s "install-docker"
```

## Install script

Containerlab can be installed using the [installation script](https://github.com/srl-labs/containerlab/blob/main/get.sh) that detects the operating system type and installs the relevant package:

/// note
Containerlab is distributed via deb/rpm packages, thus only Debian- and RHEL-like distributives can leverage package installation.  
Other systems can follow the [manual installation](#manual-installation) procedure.
///

/// tab | Latest release

Download and install the latest release (may require `sudo`):
<!-- --8<-- [start:install-script-cmd] -->
```{.bash .no-select}
bash -c "$(curl -sL https://get.containerlab.dev)"
```
<!-- --8<-- [end:install-script-cmd] -->
///

/// tab | Specific version

Download a specific version. Versions can be found on the [Releases](https://github.com/srl-labs/containerlab/releases) page.

```bash
bash -c "$(curl -sL https://get.containerlab.dev)" -- -v 0.10.3
```

///

/// tab | with `wget`

```bash
# with wget
bash -c "$(wget -qO - https://get.containerlab.dev)"
```

///

Since the installation script uses GitHub API, users may hit the rate limit imposed by GitHub. To avoid this, users can pass their personal GitHub token as an env var to the installation script:

```bash
GITHUB_TOKEN=<your token> bash -c "$(curl -sL https://get.containerlab.dev)"
```

## Package managers

It is possible to install official containerlab releases via public APT/YUM repository.

/// tab | APT

```bash
echo "deb [trusted=yes] https://netdevops.fury.site/apt/ /" | \
sudo tee -a /etc/apt/sources.list.d/netdevops.list

sudo apt update && sudo apt install containerlab
```

///
/// tab | YUM

```
yum-config-manager --add-repo=https://netdevops.fury.site/yum/ && \
echo "gpgcheck=0" | sudo tee -a /etc/yum.repos.d/yum.fury.io_netdevops_.repo

sudo yum install containerlab
```

///

/// tab | APK
Download `.apk` package from [Github releases](https://github.com/srl-labs/containerlab/releases).
///

/// tab | AUR
Arch Linux users can download a package from this [AUR repository](https://aur.archlinux.org/packages/containerlab-bin).
///

/// details | Manual package installation
Alternatively, users can manually download the deb/rpm package from the [Github releases](https://github.com/srl-labs/containerlab/releases) page.

example:

```bash
# manually install latest release with package managers
LATEST=$(curl -s https://github.com/srl-labs/containerlab/releases/latest | sed -e 's/.*tag\/v\(.*\)\".*/\1/')
# with yum
yum install "https://github.com/srl-labs/containerlab/releases/download/v${LATEST}/containerlab_${LATEST}_linux_amd64.rpm"
# with dpkg
curl -sL -o /tmp/clab.deb "https://github.com/srl-labs/containerlab/releases/download/v${LATEST}/containerlab_${LATEST}_linux_amd64.deb" && dpkg -i /tmp/clab.deb

# install specific release with yum
yum install https://github.com/srl-labs/containerlab/releases/download/v0.7.0/containerlab_0.7.0_linux_386.rpm
```

///
The package installer will put the `containerlab` binary in the `/usr/bin` directory as well as create the `/usr/bin/clab -> /usr/bin/containerlab` symlink. The symlink allows the users to save on typing when they use containerlab: `clab <command>`.

## Container

Containerlab is also available in a container packaging. The latest containerlab release can be pulled with:

```
docker pull ghcr.io/srl-labs/clab
```

To pick any of the released versions starting from release 0.19.0, use the version number as a tag, for example, `docker pull ghcr.io/srl-labs/clab:0.19.0`

Since containerlab itself deploys containers and creates veth pairs, its run instructions are a bit more complex, but still, it is a copy-paste-able command.

For example, if your lab files are contained within the current working directory - `$(pwd)` - then you can launch containerlab container as follows:

```bash
docker run --rm -it --privileged \
    --network host \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /var/run/netns:/var/run/netns \
    -v /etc/hosts:/etc/hosts \
    -v /var/lib/docker/containers:/var/lib/docker/containers \
    --pid="host" \
    -v $(pwd):$(pwd) \
    -w $(pwd) \
    ghcr.io/srl-labs/clab bash
```

Within the started container you can use the same `containerlab deploy/destroy/inspect` commands to manage your labs.

/// note
Containerlab' container command is itself `containerlab`, so you can deploy a lab without invoking a shell, for example:

```bash
docker run --rm -it --privileged \
# <run options omitted>
-w $(pwd) \
ghcr.io/srl-labs/clab deploy -t somelab.clab.yml
```

///

## Manual installation

If the linux distributive can't install deb/rpm packages, containerlab can be installed from the archive:

```bash
# get the latest available tag
LATEST=$(curl -s https://github.com/srl-labs/containerlab/releases/latest | \
       sed -e 's/.*tag\/v\(.*\)\".*/\1/')

# download tar.gz archive
curl -L -o /tmp/clab.tar.gz "https://github.com/srl-labs/containerlab/releases/download/v${LATEST}/containerlab_${LATEST}_Linux_amd64.tar.gz"

# create containerlab directory
mkdir -p /etc/containerlab

# extract downloaded archive into the containerlab directory
tar -zxvf /tmp/clab.tar.gz -C /etc/containerlab

# (optional) move containerlab binary somewhere in the $PATH
mv /etc/containerlab/containerlab /usr/bin && chmod a+x /usr/bin/containerlab
```

## Windows Subsystem Linux (WSL)

Containerlab [runs](https://twitter.com/ntdvps/status/1380915270328401922) on WSL, but you need to [install docker-ce](https://docs.docker.com/engine/install/) inside the WSL2 linux system instead of using Docker Desktop[^3].

If you are running Ubuntu/Debian as your WSL2 machine, you can use the [quick setup this script](https://github.com/srl-labs/containerlab/blob/main/utils/quick-setup.sh) to install docker-ce.

```bash
curl -L https://containerlab.dev/setup | sudo bash -s "install-docker" | \
```

Once installed, issue `sudo service docker start` to start the docker service inside WSL2 machine.

/// details | Running VM-based routers inside WSL
In Windows 11 with WSL2 it is now possible to [enable KVM support](https://serverfault.com/a/1115773/351978). Let us know if that worked for you in our [Discord](community.md).
///

## Apple macOS

Running containerlab on macOS is possible both on ARM (M1/M2) and Intel chipsets with certain limitations and caveats rooted in different architectures and underlying OS.

### ARM

At the moment of this writing, there are not a lot[^6] of Network OSes built for arm64 architecture. This fact alone makes it not practical to run containerlab natively on ARM-based Macs. Nevertheless, it is technically possible to run containerlab on ARM-based Macs by launching a Linux VM with x86_64 architecture and running containerlab inside this VM. This approach comes with a hefty performance penalty, therefore it is suitable only for tiny labs.

#### UTM

The easiest way to start a Linux VM with x86_64 architecture on macOS is to use [UTM](https://mac.getutm.app/). UTM is a free[^7] and open-source graphical virtual machine manager that provides a simple and intuitive interface for creating, managing, and running virtual machines with qemu.

When you have UTM installed, you can download a pre-built Debian 12 UTM image built by the Containerlab team using the following command[^8]:

```bash
sudo docker run --rm -v $(pwd):/workspace ghcr.io/oras-project/oras:v1.1.0 pull \
    ghcr.io/srl-labs/containerlab/clab-utm-box:0.1.0
```

By running this command you will download the `clab_debian12.utm` file which is a UTM image with `containerlab`, `docker-ce` and `gh` tools pre-installed[^9].

Open the downloaded image with UTM **File -> Open -> select .utm file** and start the VM.

Once the VM is started, you can log in using `debian:debian` credentials. Run `ip -4 addr` in the terminal to find out which IP got assigned to this VM.  
Now you can use this IP for your Mac terminal to connect to the VM via SSH[^10].

When logged in, you can upgrade the containerlab to the latest version with:

```bash
sudo clab version upgrade
```

and start downloading the labs you want to run.

#### Docker in Docker

Another option to run containerlab on ARM-based Macs is to use Docker in Docker approach. With this approach, a docker-in-docker container is launched on the macOS inside the VM providing a docker environment. This setup also works on other operating systems where Docker is available. Below is a step-by-step guide on how to set it up.

//// details | "Docker in docker guide"
We'll provide an example of a custom [devcontainer](https://code.visualstudio.com/docs/devcontainers/containers) that can be opened in [VSCode](https://code.visualstudio.com) with [Remote Development extension pack](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.vscode-remote-extensionpack) installed.

Create `.devcontainer` directory in the root of the Containerlab repository with the following content:

```text
.devcontainer
|- devcontainer.json
|- Dockerfile
```

/// tab | Dockerfile

```Dockerfile
# The devcontainer will be based on debian bullseye
# The base container already has entrypoint, vscode user account, etc. out of the box
FROM mcr.microsoft.com/vscode/devcontainers/base:bullseye

# containelab version will be set in devcontainer.json
ARG _CLAB_VERSION

# Set permissions for mounts in devcontainer.json
RUN mkdir -p /home/vscode/.vscode-server/bin
RUN chown -R vscode:vscode /home/vscode/.vscode-server

# install some basic tools inside the container
# adjust this list based on your demands
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
    sshpass \
    curl \
    iputils-ping \
    htop \
    yamllint \
    && rm -rf /var/lib/apt/lists/* \
    && rm -Rf /usr/share/doc && rm -Rf /usr/share/man \
    && apt-get clean

# install preferred version of the containerlab
RUN bash -c "$(curl -sL https://get.containerlab.dev)" -- -v ${_CLAB_VERSION}
```

///

/// tab | devcontainer.json

```json
// For format details, see https://aka.ms/devcontainer.json. For config options, see the
// README at: https://github.com/devcontainers/templates/tree/main/src/python
{
    "name": "clab-dev-container",
    "build": {
        "dockerfile": "Dockerfile",
        "args": {
            "_CLAB_VERSION": "0.47.2"
        }
    },
    "features": {
        // Containerlab will run in a docker-in-docker container
        // it is also possible to use docker-outside-docker feature
        "ghcr.io/devcontainers/features/docker-in-docker:latest": {
            "version": "latest"
        }
        // You can add other features from this list: https://github.com/orgs/devcontainers/packages?repo_name=features
        // For example:
        //"ghcr.io/devcontainers/features/go:latest": {
        //    "version": "1.21"
        //}

    },
    // add any required extensions that must be pre-installed in the devcontainer
    "customizations": {
        "vscode": {
            "extensions": [
                // various tools
                "ms-azuretools.vscode-docker",
                "tuxtina.json2yaml",
                "vscode-icons-team.vscode-icons",
                "mutantdino.resourcemonitor"
            ]
        }
    },
    // This adds persistent mounts, so some configuration like docker credentials are saved for the vscode user and root (for sudo).
    // Furthermore, your bash history and other configurations you made in your container users 'vscode' home are saved.
    // .vscode-server is an anonymous volume. Gets destroyed on rebuild, which allows vscode to reinstall the extensions and dotfiles.
    "mounts": [
    "source=clab-vscode-home-dir,target=/home/vscode,type=volume",
    "source=clab-docker-root-config,target=/root/.docker,type=volume",
    "target=/home/vscode/.vscode-server,type=volume"
]
}
```

///
Once the devcontainer is defined as described above:

* Open the devcontainer in VSCode
* Import the required images for your cLab inside the container (if you are using Docker-in-Docker option)
* Start your Containerlab
////

### Intel

On Intel based Macs, containerlab can be run in a Linux VM started by Docker Desktop for Mac[^4]. To start using containerlab in this Linux VM we start a container with containerlab inside and mount the directory with our lab files into the container.

```shell linenums="1"
CLAB_WORKDIR=~/clab

docker run --rm -it --privileged \
    --network host \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /run/netns:/run/netns \
    --pid="host" \
    -w $CLAB_WORKDIR \
    -v $CLAB_WORKDIR:$CLAB_WORKDIR \
    ghcr.io/srl-labs/clab bash
```

The first command in the snippet above sets the working directory which you intend to use on your macOS. The `~/clab` in the example above expands to `/Users/<username>/clab` and means that we intend to have our containerlab labs to be stored in this directory.

/// note

1. It is best to create a directory under the `~/some/path` unless you know what to do[^5]
2. vrnetlab based nodes will not be able to start, since Docker VM does not support virtualization.
3. Docker Desktop for Mac introduced cgroups v2 support in 4.3.0 version; to support the images that require cgroups v1 follow [these instructions](https://github.com/docker/for-mac/issues/6073).
4. Docker Desktop relies on a LinuxKit based HyperKit VM. Unfortunately, it is shipped with a minimalist kernel, and some modules such as VRF are disabled by default. Follow [these instructions](https://medium.com/@notsinge/making-your-own-linuxkit-with-docker-for-mac-5c1234170fb1) to rebuild it with more modules.
///

When the container is started, you will have a bash shell opened with the directory contents mounted from the macOS. There you can use `containerlab` commands right away.

/// details | Step-by-step example
Let's imagine I want to run a lab with two SR Linux containers running directly on a macOS.

First, I need to have Docker Desktop for Mac installed and running.

Then I will create a directory under the `$HOME` path on my mac:

```
mkdir -p ~/clab
```

Then I will create a clab file defining my lab in the newly created directory:

```bash
cat <<EOF > ~/clab/2srl.clab.yml
name: 2srl

topology:
    nodes:
    srl1:
        kind: nokia_srlinux
        image: ghcr.io/nokia/srlinux
    srl2:
        kind: nokia_srlinux
        image: ghcr.io/nokia/srlinux

    links:
    - endpoints: ["srl1:e1-1", "srl2:e1-1"]
EOF
```

Now when the clab file is there, launch the container and don't forget to use path to the directory you created:

```bash
CLAB_WORKDIR=~/clab

docker run --rm -it --privileged \
    --network host \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /run/netns:/run/netns \
    --pid="host" \
    -w $CLAB_WORKDIR \
    -v $CLAB_WORKDIR:$CLAB_WORKDIR \
    ghcr.io/srl-labs/clab bash
```

Immediately you will get into the directory inside the container with your lab file available:

```
root@docker-desktop:/Users/romandodin/clab# ls
2srl.clab.yml
```

Now you can launch the lab, as containerlab is already part of the image:

```
root@docker-desktop:/Users/romandodin/clab# clab dep -t 2srl.clab.yml
INFO[0000] Parsing & checking topology file: 2srl.clab.yml
INFO[0000] Creating lab directory: /Users/romandodin/clab/clab-2srl
INFO[0000] Creating root CA
INFO[0000] Creating docker network: Name='clab', IPv4Subnet='172.20.20.0/24', IPv6Subnet='2001:172:20:20::/64', MTU='1500'
INFO[0000] Creating container: srl1
INFO[0000] Creating container: srl2
INFO[0001] Creating virtual wire: srl1:e1-1 <--> srl2:e1-1
INFO[0001] Adding containerlab host entries to /etc/hosts file
+---+----------------+--------------+-----------------------+------+-------+---------+----------------+----------------------+
| # |      Name      | Container ID |         Image         | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+----------------+--------------+-----------------------+------+-------+---------+----------------+----------------------+
| 1 | clab-2srl-srl1 | 574bf836fb40 | ghcr.io/nokia/srlinux | srl  |       | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
| 2 | clab-2srl-srl2 | f88531a74ffb | ghcr.io/nokia/srlinux | srl  |       | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
+---+----------------+--------------+-----------------------+------+-------+---------+----------------+----------------------+
```

///

## Upgrade

To upgrade `containerlab` to the latest available version issue the following command[^1]:

```
sudo -E containerlab version upgrade
```

This command will fetch the installation script and will upgrade the tool to its most recent version. In case of GitHub rate limit, provide `GITHUB_TOKEN` env var with your personal GitHub token to the upgrade command.

Or leverage `apt`/`yum` utilities if containerlab repo was added as explained in the [Package managers](#package-managers) section.

## From source

To build containerlab from source:

/// tab | with `go build`
To build containerlab from source, clone the repository and issue `go build` at its root.
///

/// tab | with goreleaser
When we release containerlab we use [goreleaser](https://goreleaser.com/) project to build binaries for all supported platforms as well as the deb/rpm packages.  
Users can install `goreleaser` and do the same locally by issuing the following command:

```
goreleaser --snapshot --skip-publish --rm-dist
```

///

## Uninstall

To uninstall containerlab when it was installed via installation script or packages:

/// tab | Debian-based system

```
apt remove containerlab
```

///

/// tab | RPM-based systems

```
yum remove containerlab
```

///

/// tab | Manual removal
Containerlab binary is located at `/usr/bin/containerlab`. In addition to the binary, containerlab directory with static files may be found at `/etc/containerlab`.
///

## SELinux

When SELinux set to enforced mode containerlab binary might fail to execute with `Segmentation fault (core dumped)` error. This might be because containerlab binary is compressed with [upx](https://upx.github.io/) and selinux prevents it from being decompressed by default.

To fix this:

```
sudo semanage fcontext -a -t textrel_shlib_t $(which containerlab)
sudo restorecon $(which containerlab)
```

or more globally:

```
sudo setsebool -P selinuxuser_execmod 1
```

[^1]: only available if installed from packages
[^2]: Most containerized NOS will require >1 vCPU. RAM size depends on the lab size. Architecture: AMD64. IPv6 should not be disabled in the kernel.
[^3]: No need to uninstall Docker Desktop, just make sure that it is not integrated with WSL2 machine that you intend to use with containerlab. Moreover, you can make it even work with Docker Desktop with a [few additional steps](https://twitter.com/networkop1/status/1380976461641834500/photo/1), but installing docker-ce into the WSL maybe more intuitive.
[^4]: kudos to Michael Kashin who [shared](https://github.com/srl-labs/containerlab/issues/577#issuecomment-895847387) this approach with us
[^5]: otherwise make sure to add a custom shared directory to the docker on mac.
[^6]: FRR is a good example of arm64-capable network OS. Nokia SR Linux is going to be available for arm64 in the 2024.
[^7]: There are two options to install UTM: via [downloadable dmg](https://github.com/utmapp/UTM/releases/latest/download/UTM.dmg) file (free) or App Store (paid). The App Store version is exactly the same, it is just a way to support the project.
[^8]: This command requires docker to be installed on your macOS. You can use Docker Desktop, Rancher or [colima](https://github.com/abiosoft/colima) to run docker on your macOS.
[^9]: If you want to install these tools on an existing Debian machine, you can run `wget -qO- containerlab.dev/setup-debian | bash -s -- all` command.
[^10]: The UTM image has a pre-installed ssh key for the `debian` user. You can download the shared private key from [here](https://github.com/srl-labs/clabernetes/blob/main/launcher/assets/default_id_rsa).
