|                               |                                                             |
| ----------------------------- | ----------------------------------------------------------- |
| **Description**               | A Full Meshed **X** Leaf(s), **Y** Spine(s) CLOS topology   |
| **Components**                | [Nokia SR Linux][srl]                                       |
| **Topology template file**    | [templated01.clab.gotmpl][topofile]                         |
| **Topology variable file**    | [templated01.clab_vars.yaml][topovarfile]                   |
| **Name**                      | templated01                                                 |

## Description

This lab consists of a customizable Leaf and Spine CLOS topology. The number and type of SR Linux Leaf and Spine nodes is configurable, it can be set using the topology variable file `templated01.clab_vars.yaml`.

The type of SR Linux used and the naming prefixes can be customized as well.

```yaml
spines:
  # SRL spine type
  type: ixr6
  # number of spines
  num: 2
  # prefix of spines name: ${prefix}${index}
  prefix: spine
leaves:
  # SRL leaf type
  type: ixrd3
  # number of leaves
  num: 4
  # prefix of leaf name: ${prefix}${index}
  prefix: leaf
```

<!-- <div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:7,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srl02.drawio&quot;}"></div> -->

## Configuration

```bash
clab deploy -t templated01.clab.gotmpl
```

WIP

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/templated01/templated01.clab.gotmpl
[topovarfile]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/templated01/templated01.clab_vars.yaml

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>
