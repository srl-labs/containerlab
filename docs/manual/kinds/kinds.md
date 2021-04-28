# Kinds

Containerlab launches, wires up and manages container based labs. The steps required to launch a vanilla `debian` or `centos` container image aren't at all different. On the other hand, Nokia SR Linux launching procedure is nothing like the one for Arista cEOS.

Things like required syscalls, mounted directories, entrypoints and commands to execute are all different for the containerized NOS'es. To let containerlab understand which launching sequence to use, the notion of a `kind` was introduced. Essentially `kinds` abstract away the need to understand certain setup peculiarities of different NOS'es.

Given the following [topology definition file](../topo-def-file.md), containerlab is able to know how to launch `node1` as an SR Linux container and `node2` as a cEOS one because they are associated with the kinds:

```yaml
name: srlceos01

topology:
  nodes:
    node1:
      kind: srl              # node1 is of srl kind
      type: ixrd2
      image: srlinux:20.6.3-145
      license: license.key
    node2:
      kind: ceos             # node2 is of ceos kind
      image: ceos:4.25F

  links:
    - endpoints: ["node1:e1-1", "node2:et1"]
```

Containerlab supports a fixed number of kinds. Within each predefined kind we store the necessary information that is used to launch the container successfully. The following kinds are supported or in the roadmap of containerlab:


| Name                | Kind                                  | Status    |
| ------------------- | ------------------------------------- | --------- |
| **Nokia SR Linux**  | [`srl`](srl.md)                       | supported |
| **Juniper cRPD**    | [`crpd`](crpd.md)                     | supported |
| **Arista cEOS**     | [`ceos`](ceos.md)                     | supported |
| **SONiC**           | [`sonic`](sonic-vs.md)                | supported |
| **Nokia SR OS**     | [`vr-sros`](vr-sros.md)               | supported |
| **Juniper vMX**     | [`vr-vmx`](vr-vmx.md)                 | supported |
| **Cisco XRv9k**     | [`vr-xrv9k`](vr-xrv9k.md)             | supported |
| **Cisco XRv**       | [`vr-xrv`](vr-xrv.md)                 | supported |
| **Arista vEOS**     | [`vr-veos`](vr-veos.md)               | supported |
| **Linux container** | [`linux`](linux.md)                   | supported |
| **Linux bridge**    | [`bridge`](bridge.md)                 | supported |
| **OvS bridge**      | [`ovs-bridge`](ovs-bridge.md)         | supported |
| **mysocketio node** | [`mysocketio`](../published-ports.md) | supported |

Refer to a specific kind documentation article to see the details about it.
