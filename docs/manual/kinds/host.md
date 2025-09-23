---
search:
  boost: 4
kind_code_name: host
kind_display_name: Host
---

# -{{ kind_display_name }}-
-{{ kind_display_name }}- is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).

A node of kind `-{{ kind_code_name }}-` represents the containerlab host the labs are running on. It is a special node that is implicitly used when nodes have links connected to the host - see [host links](../network.md#host-links).

But there is a use case when users might want to define the node of kind `-{{ kind_code_name }}-` explicitly in the topology. For example, when some commands need to be executed on the host for the lab to function.

In such case, the following topology definition can be used:

```yaml
h1:
  kind: -{{ kind_code_name }}-
  exec:
    - ip link set dev enp0s3 up
```

In the above example, the node `h1` is defined as a node of kind `-{{ kind_code_name }}-` and the `exec` option is used to run the command `ip link set dev enp0s3 up` in the containerlab host. Of course, the command can be any other command that is required for the lab to function.
