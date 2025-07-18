---
hide:
  - navigation
---

# Installation

Containerlab is distributed as a Linux deb/rpm/apk package for amd64 and arm64 architectures and can be installed on any Debian- or RHEL-like distributive in a matter of a few seconds.

## Pre-requisites

The following requirements must be satisfied to let containerlab tool run successfully:

* A user should have `sudo` privileges to run containerlab.
* A Linux server/VM[^1] and [Docker](https://docs.docker.com/engine/install/) installed.
* Load container images (e.g. Nokia SR Linux, Arista cEOS) that are not downloadable from a container registry. Containerlab will try to pull images at runtime if they do not exist locally.

## Quick setup

The easiest way to get started with containerlab is to use the [quick setup script](https://github.com/srl-labs/containerlab/blob/main/utils/quick-setup.sh) that installs all of the following components in one go (or allows to install them separately):

* docker (docker-ce), docker compose
* Containerlab (using the package repository)
* [`gh`](https://cli.github.com/) CLI tool

The script has been tested on the following OSes:

* Ubuntu 20.04, 22.04, 23.10, 24.04
* Debian 11, 12
* Red Hat Enterprise Linux 9
* CentOS Stream 9
* Fedora Server 40 (should work on other variants of Fedora)
* Rocky Linux 9.3, 8.8 (should work on any 9.x and 8.x release)

To install all components at once, run the following command on any of the supported OSes:
<!-- --8<-- [start:quick-setup-script-cmd] -->
```bash
curl -sL https://containerlab.dev/setup | sudo -E bash -s "all"
```
<!-- --8<-- [end:quick-setup-script-cmd] -->

By default, this will also configure sshd on the system to increase max auth tries so unknown keys don't lock ssh attempts.
This behavior can be turned off by setting the environment variable "SETUP_SSHD" to "false" **before** running the command shown above.
The environment variable can be set and exported with the command shown below.

```bash
export SETUP_SSHD="false"
```

To complete installation and enable sudo-less `docker` command execution, please run `newgrp docker` or logout and log back in.

Containerlab is also set up for sudo-less operation, and the user executing the quick install script is automatically granted access to privileged commands. For further information, see [Sudo-less operation](#sudo-less-operation).

To install an individual component, specify the function name as an argument to the script. For example, to install only `docker`:

```bash
curl -sL https://containerlab.dev/setup | sudo -E bash -s "install-docker"
```

If you don't have your own shell configuration and want to have a slightly better bash PS1 prompt you can also run this script:

```bash
curl -sL https://containerlab.dev/setup | sudo -E bash -s "setup-bash-prompt"
```

Log out and log back in to see the new two-line prompt in action:

```bash
[*]─[clab]─[~]
└──>
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
sudo yum-config-manager --add-repo=https://netdevops.fury.site/yum/ && \
echo "gpgcheck=0" | sudo tee -a /etc/yum.repos.d/netdevops.fury.site_yum_.repo

sudo yum install containerlab
```

///

//// tab | DNF4

```bash
sudo dnf config-manager -y --add-repo "https://netdevops.fury.site/yum/" && \
echo "gpgcheck=0" | sudo tee -a /etc/yum.repos.d/netdevops.fury.site_yum_.repo

sudo dnf install containerlab
```

////

//// tab | DNF5

```bash
sudo dnf config-manager addrepo --set=baseurl="https://netdevops.fury.site/yum/" && \
echo "gpgcheck=0" | sudo tee -a /etc/yum.repos.d/netdevops.fury.site_yum_.repo

sudo dnf install containerlab
```

////

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
Containerlab is also set up for sudo-less operation, and the current user (even if the package manager was called through `sudo`) is automatically granted access to privileged Containerlab commands. For further information, see [Sudo-less operation](#sudo-less-operation).

## Windows

Containerlab runs on Windows powered by Windows Subsystem Linux (aka WSL), where you can run Containerlab directly or in a Devcontainer. Open up [**Containerlab on Windows**](windows.md) documentation for more details.

## Apple macOS

Running containerlab on macOS is possible both on ARM (M1/M2/M3/etc) and Intel chipsets. For a long time, we had many caveats around M-chipsets on Macs, but with the introduction of ARM64-native NOSes like Nokia SR Linux and Arista cEOS, powered by Rosetta emulation for x86_64-based NOSes, it is now possible to run containerlab on ARM-based Macs.

Since we wanted to share our experience with running containerlab on macOS in details, we have created a separate - [**Containerlab on macOS**](macos.md) - guide.

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

## Upgrade

To upgrade `containerlab` to the latest available version issue the following command[^2]:

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

## Sudo-less operation

Containerlab requires root privileges to perform certain operations.

To simplify usage, by default, Containerlab is installed as a _SUID binary_[^3] to permit sudo-less operation.

/// details | Enabling sudo-less operations for manually built/installed Containerlab
    type: subtle-note
To enable sudo-less operation for users who who manually built and installed Containerlab, or did not use a package manager to install it, the following commands need to be run:

```bash
# Set SUID bit on Containerlab binary
sudo chmod u+s `which containerlab`
# Create clab_admins Unix group
sudo groupadd -r clab_admins
# Add current user to clab_admins group
sudo usermod -aG clab_admins "$USER"
```

Users who manage their Containerlab installation via `deb/yum/dnf` package managers will have the sudo-less functionality automatically enabled during the first upgrade from pre-`0.63.0` versions.

To check whether Containerlab is enabled for sudo-less operations, run the following commands:

```{.bash .no-select}
ls -hal `which containerlab`
```

<div class="embed-result">
```{.bash .no-select .no-copy}
-rwsr-xr-x 1 root root 131M Jan 17 17:56 /usr/bin/containerlab
#  ^ SUID bit set, owned by root
```
</div>
```{.bash .no-select}
groups
```
<div class="embed-result">
```{.bash .no-select .no-copy}
... clab_admins ...
#   ^ user is member of clab_admins group
```
</div>

///

Additionally, to prevent unauthorized users from gaining root-level privileges through Containerlab, the usage of privileged Containerlab commands is gated behind a Unix user group membership check. Privileged Containerlab commands can only be performed by users who are part of the `clab_admins` group.  
By default (starting with version `0.63.0`), the `clab_admins` Unix group is created during the initial installation of Containerlab, and the user installing Containerlab is automatically added to this user group. Additional users who require access to privileged Containerlab commands should also be added to this user group.

Users who are _not_ part of this group can still execute non-privileged commands, such as:

* exec (requires `docker` group membership)
* generate
* graph
* inspect (requires `docker` group membership)
* save
* version (no upgrade)

Non-privileged commands are executed as the user running the Containerlab commands. Privileged commands are executed as root during runtime.  
Non-privileged command execution is only supported when the default container runtime, Docker.

To allow _any user on the host_ to use all Containerlab commands, simply delete the `clab_admins` Unix group.

/// danger
Much like the `docker` group, any users part of the `clab_admins` group are effectively given root-level privileges to the system running Containerlab.

**If this group does not exist and the binary still has the SUID bit set, any user who can run Containerlab should be treated as having root privileges.**
///

To **disable sudo-less operation**, simply unset the SUID flag on the Containerlab binary:

```
sudo chmod u-s `which containerlab`
```

Containerlab installers will **not** attempt to set the SUID flag or create the `clab_admins` group as long as the empty file `/etc/containerlab/suid_setup_done` exists.  
This file is automatically created during the first installation of Containerlab `0.63.0` or newer.

[^1]: Most containerized NOS will require >1 vCPU. RAM size depends on the lab size. IPv6 should not be disabled in the kernel.
[^2]: only available if installed from packages
[^3]: SUID, or "set user ID", is a special permission bit that can be set on Unix systems. SUID binaries run as the owner of the file, rather than as the executing user.
