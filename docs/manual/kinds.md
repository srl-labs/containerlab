Containerlab launches and wires up containers. The steps required to launch a `debian` or `centos` image are not different. On the other hand, launching procedure of a Nokia SR Linux container differs a lot from the Arista cEOS requirements.

Things like required syscalls, license file or directories mounting, entrypoints and commands to execute are all different for the containerized NOS'es. For containerlab to be able to understand which launch steps to take for which node we introduced the notion of a `kind`.

Kinds define the flavor of the node, it says if the node is a specific containerized Network OS or something else.

Given the following [topology definition file](topo-def-file.md), containerlab is able to know how to launch `node1` as SR Linux container and `node2` as a cEOS one because they are associated with the kinds:

```yaml
name: srlceos01

topology:
  nodes:
    node1:
      kind: srl              # node1 is of srl kind
      type: ixrd2
      image: srlinux
      license: license.key
    node2:
      kind: ceos
      image: ceos            # node2 is of srl kind

  links:
    - endpoints: ["srl:e1-1", "ceos:eth1"]
```

Containerlab supports a fixed number of kinds. Within the each predefined kind we store the necessary information that is used to launch the container successfully. The following kinds are supported or in the roadmap:


| Name                | Kind     | Status    |
| ------------------- | -------- | --------- |
| **Nokia SR Linux**  | `srl`    | supported |
| **Arista cEOS**     | `ceos`   | supported |
| **Linux container** | `linux`  | supported |
| **Linux bridge**    | `bridge` | supported |
| **SONiC**           | `sonic`  | planned   |
| **Juniper cRPD**    | `crpd`   | planned   |
