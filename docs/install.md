Containerlab is distributed as Linux package and can be installed on any Debian- or RHEL-like distributive.

### Pre-requisites
* Have `sudo` rights on the system: `containerlab` sets some parameters in the linux system to support the various options the containers need
* [Install docker](https://docs.docker.com/engine/install/): this is used to manage the containers
* Pull the container images which you intend to run to the Linux system

### Package installation
Containerlab package can be installed using the [installer script](https://github.com/srl-wim/container-lab/blob/master/get.sh) which detects the operating system type and installs the relevant package:

```bash
# download and install the latest release
sudo curl -sL https://github.com/srl-wim/container-lab/raw/master/get.sh | \
sudo bash

# download a specific version - 0.6.0
sudo curl -sL https://github.com/srl-wim/container-lab/raw/master/get.sh | \
sudo bash -s -- -v 0.6.0
```

### Graphviz
Containerlab's `graph` command can render a topology graph. For the generation of PNG images out of the topology files `graphviz` tool needs to be installed.

If you don't want to install graphviz, just create the .dot file and use an [online graphviz tool](https://dreampuf.github.io/GraphvizOnline).
```bash
# Debian / Ubuntu
sudo apt-get install graphviz

# CentOS / Fedora / RedHat
sudo yum install graphviz
```

Note, that `graphviz` installation is optional and is only required when a user wants to generate PNG files on the system out of the generated `dot` files.