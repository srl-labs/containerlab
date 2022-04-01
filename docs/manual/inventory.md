To accommodate for smooth transition from lab deployment to subsequent automation activities, containerlab generates inventory files for different automation tools.

## Ansible
Ansible inventory is generated automatically for every lab. The inventory file can be found in the lab directory under the `ansible-inventory.yml` name.

Lab nodes are grouped under their kinds in the inventory so that the users can selectively choose the right group of nodes in the playbooks.

=== "topology file"
    ```yaml
    name: ansible
    topology:
      nodes:
        r1:
          kind: crpd
          image: crpd:latest

        r2:
          kind: ceos
          image: ceos:latest

        r3:
          kind: ceos
          image: ceos:latest

        grafana:
          kind: linux
          image: grafana/grafana:7.4.3
    ```
=== "generated ansible inventory"
    ```yaml
    all:
      children:
        crpd:
          hosts:
            clab-ansible-r1:
              ansible_host: <mgmt-ipv4-address>
        ceos:
          hosts:
            clab-ansible-r2:
              ansible_host: <mgmt-ipv4-address>
            clab-ansible-r3:
              ansible_host: <mgmt-ipv4-address>
        linux:
          hosts:
            clab-ansible-grafana:
              ansible_host: <mgmt-ipv4-address>
    ```

## Removing `ansible_host` var
If you want to use a plugin[^1] that doesn't play well with the `ansible_host` variable injected by containerlab in the inventory file, you can leverage the `ansible-no-host-var` label. The label can be set on per-node, kind, or default levels; if set, containerlab will not generate the `ansible_host` variable in the inventory for the nodes with that label.  
Note that without the `ansible_host` variable, the connection plugin will use the `inventory_hostname` and resolve the name accordingly if network reachability is needed.

=== "topology file"
    ```yaml
    name: ansible
      topology:
        defaults:
          labels:
            ansible-no-host-var: "true"
        nodes:
          node1:
          node2:
    ```
=== "generated ansible inventory"
    ```yaml
    all:
      children:
        linux:
          hosts:
            clab-ansible-node1:
            clab-ansible-node2:
    ```

## User-defined groups
Users can enforce custom grouping of nodes in the inventory by adding the `ansible-inventory` label to the node definition:

```yaml
name: custom-groups
topology:
  nodes:
    node1:
      # <some node config data>
      labels:
        ansible-group: spine
    node2:
      # <some node config data>
      labels:
        ansible-group: extra_group
```

As a result of this configuration, the generated inventory will look like this:

```yaml
  children:
    srl:
      hosts:
        clab-custom-groups-node1:
          ansible_host: 172.100.100.11
        clab-custom-groups-node2:
          ansible_host: 172.100.100.12
    extra_group:
      hosts:
        clab-custom-groups-node2:
          ansible_host: 172.100.100.12
    spine:
      hosts:
        clab-custom-groups-node1:
          ansible_host: 172.100.100.11
```

## Topology Data
Every time a user runs a `deploy` command, clab automatically exports comprehensive information about the topology into `topology-data.json` file in the lab directory.

The topology data file contains the following top-level sections, or keys:

```json
{
  "name": "<topology name>",
  "type": "clab",
  "clabconfig": {"<top-level information about the lab, like management network details>"},
  "nodes": {"<detailed information about every node in the topology, including dynamic information like management IP addresses>"},
  "links": ["<entries for every link between nopology nodes, including interface names and allocated MAC addresses>"]
}
````


=== "topology file srl02.clab.yml"
    ```yaml
    name: srl02

    topology:
      kinds:
        srl:
          type: ixr6
          image: ghcr.io/nokia/srlinux
      nodes:
        srl1:
          kind: srl
        srl2:
          kind: srl

      links:
        - endpoints: ["srl1:e1-1", "srl2:e1-1"]
    ```
=== "sample generated topology-data.json"
    ```json
    {
      "name": "srl02",
      "type": "clab",
      "clabconfig": {
        "prefix": "clab",
        "mgmt": {
          "network": "clab",
          "bridge": "br-<...>",
          "ipv4_subnet": "172.20.20.0/24",
          "ipv6_subnet": "2001:172:20:20::/64",
          "mtu": "1500",
          "external-access": true
        },
        "config-path": "<full path to a directory with srl02.clab.yml>"
      },
      "nodes": {
        "srl1": {
          "shortname": "srl1",
          "longname": "clab-srl02-srl1",
          "fqdn": "srl1.srl02.io",
          "labdir": "<full path to the lab node directory>",
          "kind": "srl",
          "nodetype": "ixr6",
          "image": "ghcr.io/nokia/srlinux",
          "user": "0:0",
          "cmd": "sudo bash -c 'touch /.dockerenv && /opt/srlinux/bin/sr_linux'",
          "mgmtipv4address": "172.20.20.2",
          "mgmtipv4prefixLength": 24,
          "mgmtipv6address": "2001:172:20:20::2",
          "mgmtipv6prefixLength": 64,
          "containerid": "<container id>",
          "nspath": "/proc/<...>/ns/net",
          "labels": {
            "clab-mgmt-net-bridge": "br-<...>",
            "clab-node-group": "",
            "clab-node-kind": "srl",
            "clab-node-lab-dir": "<full path to the lab node directory>",
            "clab-node-name": "srl1",
            "clab-node-type": "ixr6",
            "clab-topo-file": "<full path to the srl02.clab.yml file>",
            "containerlab": "srl02"
          },
          "deploymentstatus": "created"
        },
        "srl2": {
          "shortname": "srl2",
          "longname": "clab-srl02-srl2",
          "fqdn": "srl2.srl02.io",
          "labdir": "<full path to the lab node directory>",,
          "index": 1,
          "kind": "srl",
          "nodetype": "ixr6",
          "image": "ghcr.io/nokia/srlinux",
          "user": "0:0",
          "cmd": "sudo bash -c 'touch /.dockerenv && /opt/srlinux/bin/sr_linux'",
          "mgmtipv4address": "172.20.20.3",
          "mgmtipv4prefixLength": 24,
          "mgmtipv6address": "2001:172:20:20::3",
          "mgmtipv6prefixLength": 64,
          "containerid": "<container id>",
          "nspath": "/proc/<...>/ns/net",
          "labels": {
            "clab-mgmt-net-bridge": "br-<...>",
            "clab-node-group": "",
            "clab-node-kind": "srl",
            "clab-node-lab-dir": "<full path to the lab node directory>",
            "clab-node-name": "srl2",
            "clab-node-type": "ixr6",
            "clab-topo-file": "<full path to the srl02.clab.yml file>",
            "containerlab": "srl02"
          },
          "deploymentstatus": "created"
        }
      },
      "links": [
        {
          "a": {
            "node": "srl1",
            "interface": "e1-1",
            "mac": "<mac address>"
          },
          "z": {
            "node": "srl2",
            "interface": "e1-1",
            "mac": "<mac address>"
          }
        }
      ]
    }
    ```

[^1]: For example [Ansible Docker connection](https://docs.ansible.com/ansible/latest/collections/community/docker/docker_connection.html) plugin.