# About lab examples
<center><div class="mxgraph" style="max-width:100%;border:1px solid transparent;" data-mxgraph="{&quot;page&quot;:4,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/containerlab-diagrams/main/containerlab.drawio&quot;}"></div></center>
<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fsrl-wim%2Fcontainerlab-diagrams%2Fmain%2Fcontainerlab.drawio" async></script>

`containerlab` aims to provide a simple, intuitive and yet customizable way to run container based labs. To help our users to get from an "I have a bare VM" point to a running and functional lab we ship some essential lab topologies inside the `containerlab` package.

These lab examples are meant to be used as-is or as a base layer to a more customized or elaborated lab scenario. Once `containerlab` is installed, you will find the lab examples directories by the `/etc/containerlab/lab-examples` path.  Copy those directories over to your working directory to start using the provided labs.

!!!note "Container images versions"
    The provided lab examples use the images without a tag, i.e. `image: srlinux`. This means that the image with a `latest` tag must exist. A user needs to tag the image themselves if the `latest` tag is missing.

    For example: `docker tag srlinux:20.6.1-286 srlinux:latest`

The source code of the lab examples is contained within the [containerlab repo](https://github.com/srl-wim/container-lab/tree/master/lab-examples); any questions or issues regarding the provided examples can be addressed via [Github issues](https://github.com/srl-wim/container-lab/issues).

Each lab comes with a definitive description that can be found in this documentation section.