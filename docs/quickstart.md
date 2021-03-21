<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>

## Installation
Getting containerlab is as easy as it gets. Thanks to the trivial [installation](install.md) procedure it can be set up in a matter of a few seconds on any RHEL or Debian based OS[^1].

```bash
# download and install the latest release
sudo curl -sL https://get-clab.srlinux.dev | sudo bash
```

## Topology definition file
Once installed, containerlab manages the labs defined in the so-called [topology definition files](manual/topo-def-file.md). A user can write a topology definition file of their own, or use the [various examples](lab-examples/lab-examples.md) we provide within the containerlab package.

In this tutorial we will be using [one of the provided labs](lab-examples/two-srls.md) which consists of two Nokia SR Linux nodes connected one to another.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:10,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

The lab topology is defined in the [srl02.yml](https://github.com/srl-labs/containerlab/blob/master/lab-examples/srl02/srl02.yml) file. To make use of this lab example, we need to copy the corresponding files to our working directory:

```bash
# create a directory for containerlab
mkdir ~/clab-quickstart
cd ~/clab-quickstart

# copy over the srl02 lab files
cp -a /etc/containerlab/lab-examples/srl02/* .
```

Let's have a look at how this lab's topology is defined:

```yaml
# contents of srl02.yml file
# topology documentation: http://containerlab.srlinux.dev/lab-examples/two-srls/
name: srl02

topology:
  kinds:
    srl:
      type: ixr6
      image: srlinux
      license: license.key
  nodes:
    srl1:
      kind: srl
    srl2:
      kind: srl

  links:
    - endpoints: ["srl1:e1-1", "srl2:e1-1"]
```

!!!info
    A [topology definition deep-dive](manual/topo-def-file.md) document provides a complete reference of the topology definition syntax. In the quickstart we keep it short, glancing over the key components of the file.

* Each lab/topology has a `name`.
* The lab topology is defined under the `topology` element.
* Topology is a set of [`nodes`](manual/nodes.md) and [`links`](manual/topo-def-file.md#links) between them.
* The nodes are always of a certain [`kind`](manual/kinds/kinds.md). The `kind` defines the node configuration and behavior.
* Containerlab supports a fixed number of `kinds`. In the example above, the `srl` kind is one of the supported kinds and it has been provided with a few additional options in the `topology.kinds.srl` element.
* `nodes` are interconnected with `links`. Each `link` is [defined](manual/topo-def-file.md#links) by a set of `endpoints`.

## Container image
One of node's most important properties is the container [`image`](manual/nodes.md#image) they use. In the example above the container image is set under the `srl` kind.
Effectively, the nodes of `srl` kind will inherit this property and will use the `srlinux` image to boot from.

The image name follows the same rules as the images you use with, for example, Docker client or k8s pods. For example, the provided `srlinux` image name assumes that the tag of the image is `latest`.

!!!note "Container images versions"
    The provided lab examples use the images without a tag, i.e. `image: srlinux`. This means that the image with a `latest` tag must exist. A user needs to tag the image if the `latest` tag is missing.

    For example: `docker tag srlinux:20.6.1-286 srlinux:latest`

## License files
For the nodes/kinds which require a license to run (like Nokia SR Linux) the [`license`](manual/nodes.md#license) element must specify a path to a valid license file.
In the example we work with, the license path is set to `license.key` for `srl` kind.

```yaml
topology:
  kinds:
    srl:
      type: ixr6
      image: srlinux
      license: license.key
```

That means that containerlab will look for this file by the `${PWD}/license.key` path. Before deploying our lab, we need to copy the file in the `~/clab-quickstart` directory to make it available by the specified path.

## Deploying a lab
Now when we know what a basic topology file consists of, sorted out the container image name and license file, we can deploy this lab. To keep things easy and guessable, the command to deploy a lab is called [`deploy`](cmd/deploy.md).

```bash
# checking that topology and license files are present in ~/clab-quickstart
❯ ls
license.key  srl02.yml

# checking that srlinux(:latest) image is available
❯ docker image ls srlinux:latest
REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
srlinux             latest              79019d14cfc7        3 months ago        1.32GB

# start the lab deployment by referencing the topology file
containerlab deploy --topo srl02.yml
```

After a couple of seconds you will see the summary of the deployed nodes:

```
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| # |      Name       | Container ID |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
| 1 | clab-srl02-srl1 | dd5c5a8dc51a | srlinux | srl  |       | running | 172.20.20.5/24 | 2001:172:20:20::5/80 |
| 2 | clab-srl02-srl2 | b623b3957f8f | srlinux | srl  |       | running | 172.20.20.4/24 | 2001:172:20:20::4/80 |
+---+-----------------+--------------+---------+------+-------+---------+----------------+----------------------+
```

The node name presented in the summary table is the fully qualified node name, it is built using the following pattern: `clab-{{lab_name}}-{{node_name}}`.

## Connecting to the nodes
Since the topology nodes are regular containers, you can connect to them just like to any other container.

```bash
docker exec -it clab-srl02-srl1 bash
```

For containerized network OSes like Nokia SR Linux or Arista cEOS you can connect with SSH by either using the management address assigned to the container:

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
ssh admin@clab-srl02-srl1
```

The following tab view aggregates the ways to open the NOS CLI per supported device:

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
    ```

## Destroying a lab
To remove the lab, use the [`destroy`](cmd/destroy.md) command that takes a topology file as an argument:

```
containerlab destroy --topo srl02.yml
```

[^1]: Make sure to satisfy lab host [pre-requisites](install.md#pre-requisites)

## What next?
To get a broader view on the containerlab features and components, refer to the **User manual** section.

Do not forget to check out the [Lab examples](lab-examples/lab-examples.md) section where we provide complete and ready-to-run topology definition files. This is a great starting point to explore containerlab by doing.