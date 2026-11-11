---
comments: true
tags:
  - Networking
  - Command
---

# Link Impairments

Labs are meant to be a reflection of real-world scenarios. To make simulated networks exhibit real-life behavior you can set link impairments (delay, jitter, packet loss) on any link that belongs to a container node. Link impairment feature is powered by the `tools netem` command collection:

* [`tools netem set`](../cmd/tools/netem/set.md)
* [`tools netem show`](../cmd/tools/netem/show.md)
* [`tools netem reset`](../cmd/tools/netem/reset.md)

These commands allow users to set, show and reset link impairments (delay, jitter, packet loss) on any link that belongs to a container node and create labs simulating real-world network conditions.

```bash title="setting packet loss at 10% rate on eth1 interface of clab-netem-r1 node"
containerlab tools netem set -n clab-netem-r1 -i eth1 --loss 10
```

## Impairments in the topology

Impairments can also be part of the topology, so they are set whenever the lab is deployed. The `netem` key of a link endpoint takes the same impairments as `tools netem set`, which apply to the packets the endpoint's interface sends:

```yaml
topology:
  links:
    - type: veth
      endpoints:
        - node: r1
          interface: eth1
          netem:
            delay: 50ms
            jitter: 5ms
            loss: 0.5
        - node: r2
          interface: eth1
          netem:
            rate: 10000
```

| Key          | Meaning                                                               |
| ------------ | --------------------------------------------------------------------- |
| `delay`      | Time to delay the packets, e.g. `100ms`                               |
| `jitter`     | Delay variation, e.g. `10ms`; needs a `delay`, at most `2.147483647s` |
| `loss`       | Random packet loss in percent                                         |
| `rate`       | Rate limit in kbit/s                                                  |
| `corruption` | Random packet corruption in percent                                   |

Brief links take a list with the impairment of each endpoint, in the order of the endpoints. An empty entry leaves its endpoint unimpaired:

```yaml
topology:
  links:
    - endpoints: ["r1:eth1", "r2:eth1"]
      netem:
        - delay: 50ms
        - {}
```

Impairments are supported on veth links. Containerlab sets them again when a node is started or restarted, since a node's interfaces lose their impairments when it is stopped, and when the lab is reconciled with a changed topology. `tools netem` can still change them while the lab runs, until containerlab sets the topology's impairments again. Removing an impairment from the topology doesn't remove it from a running lab; use `tools netem reset` for that.
