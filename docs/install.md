Containerlab is distributed as a Linux deb/rpm package and can be installed on any Debian- or RHEL-like distributive in a matter of a few seconds.

### Pre-requisites
The following requirements must be satisfied in order to let containerlab tool run successfully:

* A user should have `sudo` privileges to run containerlab.
* [Docker](https://docs.docker.com/engine/install/) must be installed.
* Import container images (e.g. Nokia SR Linux, Arista cEOS) which are not downloadable from a container registry. Containerlab will try to pull images at runtime if they do not exist locally.

### Package installation
Containerlab package can be installed using the [installation script](https://github.com/srl-wim/container-lab/blob/master/get.sh) which detects the operating system type and installs the relevant package:

!!! note
    Containerlab is distributed via deb/rpm packages, thus only Debian- and RHEL-like distributives are supported.

```bash
# download and install the latest release
sudo curl -sL https://get-clab.srlinux.dev | sudo bash

# download a specific version - 0.6.0
sudo curl -sL https://get-clab.srlinux.dev | sudo bash -s -- -v 0.6.0
```

!!!note "Manual installation"
    If the usage of piped bash scripts is discouraged or restricted, the users can manually download the package from the [Github releases](https://github.com/srl-wim/container-lab/releases) page.

    example:
    ```
    yum install https://github.com/srl-wim/container-lab/releases/download/v0.7.0/containerlab_0.7.0_linux_386.rpm
    ```

The package installer will put the `containerlab` binary in the `/usr/bin` directory as well as create the `/usr/bin/clab -> /usr/bin/containerlab` symlink. The symlink allows the users to save on typing when they use containerlab: `clab <command>`.

### Upgrade
To upgrade `containerlab` to the latest available version issue the following command:

```
containerlab version upgrade
```

This command will fetch the installation script and will upgrade the tool to its most recent version.
