|                            |                                                                 |
| -------------------------- | --------------------------------------------------------------- |
| **Description**            | A 5-stage Clos topology with **X** Pod(s), **Y** Super Spine(s) |
| **Components**             | [Nokia SR Linux][srl]                                           |
| **Topology template file** | [templated02.clab.gotmpl][topofile]                             |
| **Topology variable file** | [templated02.clab_vars.yaml][topovarfile]                       |
| **Name**                   | templated02                                                     |

## Description

This lab consists of a customizable 5 stage Clos topology.
Each pod in this lab consists of a configurable number of fully meshed spines and leaves.
The spines in each pod are connected to a configurable number of super spines.

The topology template is rendered using the variable file shown below:

```yaml
super_spines:
  # SRL super spine type
  type: ixrd3
  # number of super spines
  num: 2
  # prefix of super spines name: ${prefix}${index}
  prefix: super-spine

pods:
  # number of pods
  num: 2
  spines:
    # SRL spine type
    type: ixrd3
    # number of spines per pod
    num: 2
    # prefix of spines name: ${prefix}${index}
    prefix: spine
  leaves:
    # SRL leaf type
    type: ixrd2l
    # number of leaves per pod
    num: 4
    # prefix of leaf name: ${prefix}${index}
    prefix: leaf

```

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/clab-lab-examples-templated.drawio&quot;}"></div>

## Configuration

```bash
clab deploy -t templated02.clab.gotmpl
```

Run `configure.sh` script to configure the lab

```bash
bash configure.sh
```

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/templated01/templated01.clab.gotmpl
[topovarfile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/templated01/templated01.clab_vars.yaml

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
