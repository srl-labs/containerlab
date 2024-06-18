---
comments: true
---

# Link Impairments

Labs are meant to be a reflection of real-world scenarios. To make simulated networks exhibit real-life behavior you can set link impairments (delay, jitter, packet loss) on any link that belongs to a container node. Link impairment feature is powered by the `tools netem` command collection:

* [`tools netem set`](../cmd/tools/netem/set.md)
* [`tools netem show`](../cmd/tools/netem/show.md)

These commands allow users to set link impairments (delay, jitter, packet loss) on any link that belongs to a container node and create labs simulating real-world network conditions.

```bash title="setting packet loss at 10% rate on eth1 interface of clab-netem-r1 node"
containerlab tools netem set -n clab-netem-r1 -i eth1 --loss 10
```
