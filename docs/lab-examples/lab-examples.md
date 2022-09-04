# About lab examples
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:4,&quot;zoom&quot;:1,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>


`containerlab` aims to provide a simple, intuitive and yet customizable way to run container based labs. To help our users get a glimpse on the features containerlab packages, we ship some essential lab topologies within the `containerlab` package.

!!!note
    The lab examples that you find on this site are merely explain the basics of containerlab. For the real-life labs built with containerlab check the [clabs.netdevops.me](https://clabs.netdevops.me) catalog, where comprehensive labs are posted by the community members.

These lab examples are meant to be used as-is or as a base layer to a more customized or elaborated lab scenarios. Once `containerlab` is installed, you will find the lab examples directories by the `/etc/containerlab/lab-examples` path.  Copy those directories over to your working directory to start using the provided labs.

!!!note "Container images versions"
    Some lab examples may use the images without a tag, i.e. `image: srlinux`. This means that the image with a `latest` tag must exist. A user needs to tag the image themselves if the `latest` tag is missing.

    For example: `docker tag srlinux:20.6.1-286 srlinux:latest`

The source code of the lab examples is contained within the [containerlab repo](https://github.com/srl-labs/containerlab/tree/main/lab-examples) unless mentioned otherwise; any questions, issues or contributions related to the provided examples can be addressed via [Github issues](https://github.com/srl-labs/containerlab/issues).

Each lab comes with a definitive description that can be found in this documentation section.

## How to deploy a lab from the lab catalog?
Running the labs from the catalog is easy.

#### Copy lab catalog
First, you need to copy the lab catalog to some place, for example to a current working directory. By copying labs from their original place we ensure that the changes we might make to the lab files will not be overwritten once we upgrade containerlab. To copy the entire catalog into your working directory:

```bash
# copy over the srl02 lab files
cp -a /etc/containerlab/lab-examples/* .
```

as a result of this command you will get several directories copied to the current working directory.

!!!note Labs stored outside of containerlab
    Some big labs or community provided labs are typically stored in a separate git repository. To fetch those labs you will need to clone the lab' repo instead of copying the directories from `/etc/containerlab/lab-examples`.

#### Get the lab name
Every lab in the catalog has a unique short name. For example [this lab](two-srls.md) states in the summary table that it's name is `srl02`. You will find a folder matching this name in your working directory, change into it:
```bash
cd srl02
```

#### Check images and licenses
Within the lab directory you will find the files that are used in the lab. Usually, only the [topology definition file](../manual/topo-def-file.md) and, sometimes, config files are present in the lab directory.

If you check the topology file you will see if any license files are required and what images are specified for each node/kind.

Either change the topology file to point to the right image/license or change the image/license to match the topo definition file values.

#### Deploy the lab
You are ready to deploy!

```bash
containerlab deploy -t <topo-file>
```

#### SSH access
For nodes that come up with `ssh` enabled, the following lines can be added to the `~/.ssh/config` file on the containerlab host system to simplify access and prevent future ssh key warnings:

```
Host clab-*
  User root
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
```

## Public clab catalogs
As mentioned in the introduction of this article, the lab examples shipped with containerlab explain the features containerlab offers. The comprehensive lab examples are not part of containerlab installation as we want the community to own their work.

Some well-known catalogs of clab based labs and/or individual submissions:

* [clabs.netdevops.me](https://clabs.netdevops.me)