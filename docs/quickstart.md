---
hide:
  - navigation
---
# Quickstart

## Installation

Getting containerlab is as easy as it gets. Thanks to the trivial [installation](install.md) procedure it can be set up in a matter of a few seconds on any RHEL or Debian based OS[^1].

--8<-- "docs/install.md:install-script-cmd"

## Topology definition file

Once installed, containerlab manages the labs defined in the so-called topology definition, [`clab` files](manual/topo-def-file.md). A user can write a topology definition file from scratch, or look at [various lab examples](lab-examples/lab-examples.md) we provided within the containerlab package or explore dozens of labs our [community has shared](https://github.com/topics/clab-topo).

In this quickstart we will be using [one of the provided labs](lab-examples/srl-ceos.md) which consists of [Nokia SR Linux](manual/kinds/srl.md) and [Arista cEOS](manual/kinds/ceos.md) nodes connected one to another.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlceos01.drawio&quot;}"></div>

The lab topology is defined in the [srlceos01.clab.yml](https://github.com/srl-labs/containerlab/blob/main/lab-examples/srlceos01/srlceos01.clab.yml) file. To make use of this lab example, we need to fetch the clab file:

```bash
mkdir ~/clab-quickstart #(1)!
cd ~/clab-quickstart

curl -LO \
https://raw.githubusercontent.com/srl-labs/containerlab/main/lab-examples/srlceos01/srlceos01.clab.yml #(2)!
```

1. Create a directory to store the lab definition file.
2. Download the lab definition file.

Let's have a look at how this lab's topology is defined:

```yaml
--8<-- "https://raw.githubusercontent.com/srl-labs/containerlab/main/lab-examples/srlceos01/srlceos01.clab.yml"
```

A [topology definition deep-dive](manual/topo-def-file.md) document provides a complete reference of the topology definition syntax. In this quickstart we keep it short, glancing over the key components of the file:

* Each lab has a `name`.
* The lab topology is defined under the `topology` element.
* Topology is a set of [`nodes`](manual/nodes.md) and [`links`](manual/topo-def-file.md#links) between them.
* The nodes are always of a certain [`kind`](manual/kinds/index.md). The `kind` defines the node configuration and behavior.
* Containerlab supports a fixed number of `kinds`. In the example above, the `srl` and `ceos` are one of the [supported kinds](manual/kinds/index.md).
* The actual [nodes](manual/nodes.md) of the topology are defined in the `nodes` section which holds a map of node names. In the example above, nodes with names `srl` and `ceos` are defined.
* Node elements must have a `kind` parameter to indicate which kind this node belongs to. Under the nodes section you typically provide node-specific parameters. This lab uses a node-specific parameters - [`image`](manual/nodes.md#image).  
* `nodes` are interconnected with `links`. Each `link` is [defined](manual/topo-def-file.md#links) by a set of `endpoints`.

## Container image

One of node's most important properties is the container [`image`](manual/nodes.md#image) that the nodes are defined with. The image name follows the same rules as the images you use with, for example, Docker CLI or Docker Compose.

/// details | Image name formats and fully qualified names
    type: tip
There are several forms of how one can write an image name, but each name essentially maps to a fully qualified image name that consists of:

1. **registry** - the registry where the image is stored.
2. **organisation** - the organisation name in that registry
3. **repository** - the repository name
4. **tag** - the image tag

Below you will find different image names you can come across when working with containerlab and the corresponding FQDN names they map to:

1. `ghcr.io/nokia/srlinux`
    * registry: `ghcr.io`
    * organisation: `nokia`
    * repository: `srlinux`
    * tag: when no explicit tag is set, the implicit `latest` is used
2. `ceos:4.32.0F`
    * registry: when registry is not set, implicit `docker.io` registry is assumed
    * organisation: when none is set, implicit `library` organisation is assumed
    * repository: `ceos`
    * tag: `4.32.0F`
3. `prom/prometheus:v2.47`
    * registry: when registry is not set, implicit `docker.io` registry is assumed
    * organisation: `prom`
    * repository: `prometheus`
    * tag: `v2.47`

///

Our topology file defines two nodes, where each node is defined with its own container image:

* `ghcr.io/nokia/srlinux:24.3.3` - for Nokia SR Linux node
* `ceos:4.32.0F` - for Arista cEOS node

When containerlab starts to deploy the lab, it will first check if the images are available locally. Local images can be listed with `docker images` command.

Containerlab will compare the image names from the topology file with the local images and if the images are not available locally, it will try to pull them from the remote registry.

/// admonition | Images availability
    type: warning
Quickstart lab includes Nokia SR Linux and Arista cEOS images. While Nokia SR Linux is a publicly available image and can be pulled by anyone, its counterpart, Arista cEOS image, is not available in a public registry.

--8<-- "docs/manual/kinds/ceos.md:ceos-get-image"
///

## Deploying a lab

Now when we know what a basic topology file consists of, refreshed our knowledge on what container image name is and imported cEOS image, we can proceed with deploying this lab. To keep things easy and guessable, the command to deploy a lab is called [`deploy`](cmd/deploy.md).

Doesn't hurt to verify that we have cEOS image imported before we hit the deploy command:

```bash hl_lines="3"
docker images | grep ceos
REPOSITORY                        TAG        IMAGE ID       CREATED         SIZE
ceos                              4.32.0F    40d39e1a92c2   24 hours ago    2.4GB
```

/// admonition | Remote topology files
    attrs: {class: inline end tip}
Containerlab allows to deploy labs from files located in remote Git repositories and/or HTTP URLs. Check out deploy command [documentation](cmd/deploy.md#remote-topology-files) for more details.
///

While you can pre-pull the Nokia SR Linux image, containerlab will do it for you if it's not available locally, handy! So we are ready to deploy:

```bash
sudo containerlab deploy # (1)!
```

1. `deploy` command will automatically lookup a file matching the `*.clab.y*ml` patter to select it.  
  If you have several files and want to pick a specific one, use `--topo <path>` flag.

In no time you will see the summary table with the deployed lab nodes.  
The table will show the node name (which equals to container name), node kind, image name and a bunch of other usefule information. You can always list the nodes of the lab with [`containerlab inspect`](cmd/inspect.md) command.

```
+---+---------------------+--------------+-----------------------+---------------+---------+-----------------+----------------------+
| # |        Name         | Container ID |         Image         |     Kind      |  State  |  IPv4 Address   |     IPv6 Address     |
+---+---------------------+--------------+-----------------------+---------------+---------+-----------------+----------------------+
| 1 | clab-srlceos01-ceos | 6ec1b1367a77 | ceos:4.32.0F          | arista_ceos   | running | 172.20.20.11/24 | 2001:172:20:20::b/64 |
| 2 | clab-srlceos01-srl  | 6af1e33f4573 | ghcr.io/nokia/srlinux | nokia_srlinux | running | 172.20.20.10/24 | 2001:172:20:20::a/64 |
+---+---------------------+--------------+-----------------------+---------------+---------+-----------------+----------------------+
```

## Connecting to the nodes

We know you want to get your hands dirty with the nodes, so let's connect to them. The common way netengs use to interact with network devices is via CLI. With Network OSes you can use SSH to connect to the CLI by either using the management address assigned to the container or a node name:

```text
❯ ssh admin@clab-srlceos01-srl

Using configuration file(s): []
Welcome to the srlinux CLI.
Type 'help' (and press <ENTER>) if you need any help using this.
--{ running }--[  ]--
A:srl#
```

/// note
For each supported kind we document the management interfaces and the ways to leverage them.  
For example, `srl` kind documentation [provides](manual/kinds/srl.md) the commands to leverage SSH and gNMI interfaces.  
`ceos` kind has its own [instructions](manual/kinds/ceos.md).
///

The following tab view aggregates the ways to get CLI access per the lab node:

Since the topology nodes are regular containers, you can connect to them just like to any other container.

/// tab | Nokia SR Linux

```bash
# access CLI
docker exec -it clab-srlceos01-srl sr_cli
# access bash
docker exec -it clab-srlceos01-srl bash
```

///
/// tab | Arista cEOS

```bash
# access CLI
docker exec -it clab-srlceos01-ceos Cli
# access bash
docker exec -it clab-srlceos01-ceos bash
```

///

Feel free to explore the nodes, configure them, and run your favorite network protocols. If you break something, you can always destroy the lab and start over. Speaking of which...

## Destroying a lab

To remove the lab, use the [`destroy`](cmd/destroy.md) command that takes a topology file as an argument:

```
sudo containerlab destroy
```

## What next?

To get a broader view on the containerlab features and components, refer to the **User manual** section.

Do not forget to check out the [Lab examples](lab-examples/lab-examples.md) section where we provide complete and ready-to-run topology definition files. This is a great starting point to explore containerlab by doing.

[^1]: For other installation options such as package managers, manual binary downloads or instructions to get containerlab for non-RHEL/Debian distros, refer to the [installation guide](install.md).

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
