Containerlab is distributed as a Linux deb/rpm package and can be installed on any Debian- or RHEL-like distributive in a matter of a few seconds.

### Pre-requisites
The following requirements must be satisfied in order to let containerlab tool run successfully:

* A user should have `sudo` privileges to run containerlab.
* [Docker](https://docs.docker.com/engine/install/) must be installed.
* Import container images (e.g. Nokia SR Linux, Arista cEOS) which are not downloadable from a container registry. Containerlab will try to pull images at runtime if they do not exist locally.

### Install script
Containerlab can be installed using the [installation script](https://github.com/srl-wim/container-lab/blob/master/get.sh) which detects the operating system type and installs the relevant package:

!!! note
    Containerlab is distributed via deb/rpm packages, thus only Debian- and RHEL-like distributives are supported.

```bash
# download and install the latest release (may require sudo)
bash -c "$(curl -sL https://get-clab.srlinux.dev)"

# download a specific version - 0.10.3 (may require sudo)
bash -c "$(curl -sL https://get-clab.srlinux.dev)" -- -v 0.10.3

# with wget
bash -c "$(wget -qO - https://get-clab.srlinux.dev)"
```

### Package managers
It is possible to install official containerlab releases via public APT/YUM repository.

=== "APT"
    ```bash
    echo "deb [trusted=yes] https://apt.fury.io/netdevops/ /" | \
    sudo tee -a /etc/apt/sources.list.d/netdevops.list

    apt update && apt install containerlab
    ```
=== "YUM"
    ```
    yum-config-manager --add-repo=https://yum.fury.io/netdevops/ && \
    echo "gpgcheck=0" | sudo tee -a /etc/yum.repos.d/yum.fury.io_netdevops_.repo

    yum install containerlab
    ```

???note "Alternative installation options"
    Alternatively, users can manually download the deb/rpm package from the [Github releases](https://github.com/srl-wim/container-lab/releases) page.

    example:
    ```bash
    # manually install latest release with package managers
    LATEST=$(curl -s https://github.com/srl-wim/container-lab/releases/latest | sed -e 's/.*tag\/v\(.*\)\".*/\1/')
    # with yum
    yum install "https://github.com/srl-wim/container-lab/releases/download/v${LATEST}/containerlab_${LATEST}_linux_amd64.rpm"
    # with dpkg
    curl -sL -o /tmp/clab.deb "https://github.com/srl-wim/container-lab/releases/download/v${LATEST}/containerlab_${LATEST}_linux_amd64.deb" && dpkg -i /tmp/clab.deb

    # install specific release with yum
    yum install https://github.com/srl-wim/container-lab/releases/download/v0.7.0/containerlab_0.7.0_linux_386.rpm
    ```

The package installer will put the `containerlab` binary in the `/usr/bin` directory as well as create the `/usr/bin/clab -> /usr/bin/containerlab` symlink. The symlink allows the users to save on typing when they use containerlab: `clab <command>`.

### Upgrade
To upgrade `containerlab` to the latest available version issue the following command:

```
containerlab version upgrade
```

This command will fetch the installation script and will upgrade the tool to its most recent version.

or leverate `apt`/`yum` utilities if containerlab repo was added as explained in the [Package managers](#package-managers) section.