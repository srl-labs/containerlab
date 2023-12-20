---
hide:
  - navigation
---
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

## Installation

Getting containerlab is as easy as it gets. Thanks to the trivial [installation](install.md) procedure it can be set up in a matter of a few seconds on any RHEL or Debian based OS[^1].

```bash
# download and install the latest release (may require sudo)
bash -c "$(curl -sL https://get.containerlab.dev)"
```

## Topology definition file

Once installed, containerlab manages the labs defined in the so-called topology definition, [`clab` files](manual/topo-def-file.md). A user can write a topology definition file from scratch, or start with looking at [various lab examples](lab-examples/lab-examples.md) we provide within the containerlab package.

In this quickstart we will be using [one of the provided labs](lab-examples/srl-ceos.md) which consists of Nokia SR Linux and Arista cEOS nodes connected one to another.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlceos01.drawio&quot;}"></div>

The lab topology is defined in the [srlceos01.clab.yml](https://github.com/srl-labs/containerlab/blob/main/lab-examples/srlceos01/srlceos01.clab.yml) file. To make use of this lab example, we first may want to copy the corresponding lab files to some directory:

```bash
# create a directory for the lab
mkdir ~/clab-quickstart
cd ~/clab-quickstart

# copy over the lab files
cp -a /etc/containerlab/lab-examples/srlceos01/* .
```

Let's have a look at how this lab's topology is defined:

```yaml
name: srlceos01

topology:
  nodes:
    srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
    ceos:
      kind: ceos
      image: ceos:4.25.0F

  links:
    - endpoints: ["srl:e1-1", "ceos:eth1"]
```

A [topology definition deep-dive](manual/topo-def-file.md) document provides a complete reference of the topology definition syntax. In this quickstart we keep it short, glancing over the key components of the file:

* Each lab has a `name`.
* The lab topology is defined under the `topology` element.
* Topology is a set of [`nodes`](manual/nodes.md) and [`links`](manual/topo-def-file.md#links) between them.
* The nodes are always of a certain [`kind`](manual/kinds/index.md). The `kind` defines the node configuration and behavior.
* Containerlab supports a fixed number of `kinds`. In the example above, the `srl` and `ceos` are one of the [supported kinds](manual/kinds/index.md).
* The actual [nodes](manual/nodes.md) of the topology are defined in the `nodes` section which holds a map of node names. In the example above, nodes with names `srl` and `ceos` are defined.
* Node elements must have a `kind` parameter to indicate which kind this node belongs to. Under the nodes section you typically provide node-specific parameters. This lab uses a node-specific parameters - `image`.  
* `nodes` are interconnected with `links`. Each `link` is [defined](manual/topo-def-file.md#links) by a set of `endpoints`.

## Container image

One of node's most important properties is the container [`image`](manual/nodes.md#image) they use. In our example the nodes use a specific image which we imported upfront[^2].

The image name follows the same rules as the images you use with, for example, Docker client.

!!!note "Container images versions"
    Some lab examples use the images without a tag, i.e. `image: srlinux`. This means that the image with a `latest` tag must exist. A user needs to tag the image if the `latest` tag is missing.

    For example: `docker tag srlinux:20.6.1-286 srlinux:latest`

!!!warning "Images availability"
    Quickstart lab includes Nokia SR Linux and Arista cEOS images. While Nokia SR Linux is a publicly available image and can be pulled by anyone, its counterpart Arista cEOS images needs to be downloaded by the users first.

    This means that you have to login with Arista website and download the image, then import it to docker image store before proceeding with this lab. Or you can swap the ceos image with another SR Linux image and enjoy the freedom of labbing.

## Deploying a lab

Now when we know what a basic topology file consists of and sorted out the container image name and node's license file, we can proceed with deploying this lab. To keep things easy and guessable, the command to deploy a lab is called [`deploy`](cmd/deploy.md).

```bash
# checking that topology file is present in ~/clab-quickstart
❯ ls
srlceos01.clab.yml

# checking that container images are available
docker images | grep -E "srlinux|ceos"
REPOSITORY             TAG                 IMAGE ID            CREATED             SIZE
ghcr.io/nokia/srlinux  latest              79019d14cfc7        3 months ago        1.32GB
ceos                   4.25.0F             15a5f97fe8e8        3 months ago        1.76GB

# start the lab deployment
containerlab deploy # (1)!
```

1. `deploy` command will automatically lookup a file matching the `*.clab.y*ml` patter to select it.  
  If you have several files and want to pick a specific one, use `--topo <path>` flag.

!!!tip "Remote topology files"
    Containerlab allows to deploy labs from files located in remote Git repositories and/or HTTP URLs. Check out deploy command [documentation](cmd/deploy.md#remote-topology-files) for more details.

After a couple of seconds you will see the summary of the deployed nodes:

```
+---+---------------------+--------------+-----------------------+------+-------+---------+----------------+----------------------+
| # |        Name         | Container ID |       Image           | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+---------------------+--------------+-----------------------+------+-------+---------+----------------+----------------------+
| 1 | clab-srlceos01-ceos | 2e2e04a42cea | ceos:4.25.0F          | ceos |       | running | 172.20.20.3/24 | 2001:172:20:20::3/80 |
| 2 | clab-srlceos01-srl  | 1b9568fcdb01 | ghcr.io/nokia/srlinux | srl  |       | running | 172.20.20.4/24 | 2001:172:20:20::4/80 |
+---+---------------------+--------------+-----------------------+------+-------+---------+----------------+----------------------+
```

The node name presented in the summary table is the fully qualified node name, it is built using the following pattern: `clab-{{lab-name}}-{{node-name}}`.

## Connecting to the nodes

Since the topology nodes are regular containers, you can connect to them just like to any other container.

```bash
docker exec -it clab-srlceos01-srl1 bash
```

!!!info
    For each supported kind we document the management interfaces and the ways to leverage them.  
    For example, `srl` kind documentation [provides](manual/kinds/srl.md) the commands to leverage SSH and gNMI interfaces.  
    `ceos` kind has its own [instructions](manual/kinds/ceos.md).

With containerized network OSes like [Nokia SR Linux](manual/kinds/srl.md) or Arista cEOS SSH access can be achieved by either using the management address assigned to the container:

```text
❯ ssh admin@172.20.20.3
admin@172.20.20.3's password:
Using configuration file(s): []
Welcome to the srlinux CLI.
Type 'help' (and press <ENTER>) if you need any help using this.
--{ running }--[  ]--
A:srl1#
```

or by using node's fully qualified names, for which containerlab creates `/etc/hosts` entries:

```
ssh admin@clab-srlceos01-srl
```

The following tab view aggregates the ways to get CLI access per the lab node:

=== "Nokia SR Linux"
    ```bash
    # access CLI
    docker exec -it <name> sr_cli
    # access bash
    docker exec -it <name> bash
    ```
=== "Arista cEOS"
    ```bash
    # access CLI
    docker exec -it <name> Cli
    # access bash
    docker exec -it <name> bash
    ```

## Destroying a lab

To remove the lab, use the [`destroy`](cmd/destroy.md) command that takes a topology file as an argument:

```
containerlab destroy --topo srlceos01.clab.yml
```

## What next?

To get a broader view on the containerlab features and components, refer to the **User manual** section.

Do not forget to check out the [Lab examples](lab-examples/lab-examples.md) section where we provide complete and ready-to-run topology definition files. This is a great starting point to explore containerlab by doing.

[^1]: For other installation options such as package managers, manual binary downloads or instructions to get containerlab for non-RHEL/Debian distros, refer to the [installation guide](install.md).
[^2]: Containerlab would try to pull the images upon the lab deployment, but if the images are not available publicly or you do not have the private repositories configured, then you need to import the images upfront.
