To accommodate for smooth transition from lab deployment to subsequent automation activities, containerlab generates inventory files for different automation tools.

## Ansible

Ansible inventory is generated automatically for every lab. The inventory file can be found in the lab directory under the `ansible-inventory.yml` name.

Lab nodes are grouped under their kinds in the inventory so that the users can selectively choose the right group of nodes in the playbooks.

///tab | topology file

```yaml
name: ansible
topology:
  nodes:
    r1:
      kind: crpd
      image: crpd:latest

    r2:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:latest

    r3:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:latest

    grafana:
      kind: linux
      image: grafana/grafana:7.4.3
```

///
///tab | generated Ansible inventory

```yaml
all:
  children:
    crpd:
      hosts:
        clab-ansible-r1:
          ansible_host: <mgmt-ipv4-address>
    nokia_srlinux:
      vars:
        ansible_network_os: nokia.srlinux.srlinux
        ansible_connection: ansible.netcommon.httpapi
      hosts:
        clab-ansible-r2:
          ansible_host: <mgmt-ipv4-address>
          ansible_user: admin
          ansible_password: NokiaSrl1!
        clab-ansible-r3:
          ansible_host: <mgmt-ipv4-address>
    linux:
      hosts:
        clab-ansible-grafana:
          ansible_host: <mgmt-ipv4-address>
```

///

For certain node kinds containerlab sets default `ansible_network_os` and `ansible_connection` variables to enable plug-and-play experience with Ansible. As well as adding username and password known to containerlab as default credentials.

### Removing `ansible_host` var

If you want to use a plugin[^1] that doesn't play well with the `ansible_host` variable injected by containerlab in the inventory file, you can leverage the `ansible-no-host-var` label. The label can be set on per-node, kind, or default levels; if set, containerlab will not generate the `ansible_host` variable in the inventory for the nodes with that label.  
Note that without the `ansible_host` variable, the connection plugin will use the `inventory_hostname` and resolve the name accordingly if network reachability is needed.

///tab | topology file

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

///
///tab | generated Ansible inventory

```yaml
all:
  children:
    linux:
      hosts:
        clab-ansible-node1:
        clab-ansible-node2:
```

///

### User-defined groups

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

Every time a user runs a `deploy` command, containerlab automatically exports information about the topology into `topology-data.json` file in the lab directory. Schema of exported data is determined based on a Go template specified in `--export-template` parameter, or a default template `/etc/containerlab/templates/export/auto.tmpl`, if the parameter is not provided.

Containerlab internal data that is submitted for export via the template, has the following structure:

```golang
type TopologyExport struct {
 Name        string                       `json:"name"`                  // Containerlab topology name
 Type        string                       `json:"type"`                  // Always 'clab'
 Clab        *CLab                        `json:"clab,omitempty"`        // Data parsed from a topology definitions yaml file
 NodeConfigs map[string]*types.NodeConfig `json:"nodeconfigs,omitempty"` // Definitions of nodes expanded with dynamically created data
}
```

To get the full list of fields available for export, you can export topology data with the following template `--export-template /etc/containerlab/templates/export/full.tmpl`. Note, some fields exported via `full.tmpl` might contain sensitive information like TLS private keys. To customize export data, it is recommended to start with a copy of `auto.tmpl` and change it according to your needs.

Example of exported data when using default `auto.tmpl` template:

=== "topology file srl02.clab.yml"
    ```yaml
    name: srl02

    topology:
      kinds:
        srl:
          type: ixrd3
          image: ghcr.io/nokia/srlinux
      nodes:
        srl1:
          kind: nokia_srlinux
        srl2:
          kind: nokia_srlinux

      links:
        - endpoints: ["srl1:e1-1", "srl2:e1-1"]
    ```
=== "sample generated topology-data.json"
    ```json
    {
      "name": "srl02",
      "type": "clab",
      "clab": {
        "config": {
          "prefix": "clab",
          "mgmt": {
            "network": "clab",
            "bridge": "br-<...>",
            "ipv4-subnet": "172.20.20.0/24",
            "ipv6-subnet": "3fff:172:20:20::/64",
            "mtu": "1500",
            "external-access": true
          },
          "config-path": "<full path to a directory with srl02.clab.yml>"
        }
      },
      "nodes": {
        "srl1": {
          "index": "0",
          "shortname": "srl1",
          "longname": "clab-srl02-srl1",
          "fqdn": "srl1.srl02.io",
          "group": "",
          "labdir": "<full path to the lab node directory>",
          "kind": "srl",
          "image": "ghcr.io/nokia/srlinux",
          "mgmt-net": "",
          "mgmt-intf": "",
          "mgmt-ipv4-address": "172.20.20.3",
          "mgmt-ipv4-prefix-length": 24,
          "mgmt-ipv6-address": "3fff:172:20:20::3",
          "mgmt-ipv6-prefix-length": 64,
          "mac-address": "",
          "labels": {
            "clab-mgmt-net-bridge": "br-<...>",
            "clab-node-group": "",
            "clab-node-kind": "srl",
            "clab-node-lab-dir": "<full path to the lab node directory>",
            "clab-node-name": "srl1",
            "clab-node-type": "ixrd3",
            "clab-topo-file": "<full path to the srl02.clab.yml file>",
            "containerlab": "srl02"
          }
        },
        "srl2": {
          "index": "1",
          "shortname": "srl2",
          "longname": "clab-srl02-srl2",
          "fqdn": "srl2.srl02.io",
          "group": "",
          "labdir": "<full path to the lab node directory>",
          "kind": "srl",
          "image": "ghcr.io/nokia/srlinux",
          "mgmt-net": "",
          "mgmt-intf": "",
          "mgmt-ipv4-address": "172.20.20.2",
          "mgmt-ipv4-prefix-length": 24,
          "mgmt-ipv6-address": "3fff:172:20:20::2",
          "mgmt-ipv6-prefix-length": 64,
          "mac-address": "",
          "labels": {
            "clab-mgmt-net-bridge": "br-<...>",
            "clab-node-group": "",
            "clab-node-kind": "srl",
            "clab-node-lab-dir": "<full path to the lab node directory>",
            "clab-node-name": "srl2",
            "clab-node-type": "ixrd3",
            "clab-topo-file": "<full path to the srl02.clab.yml file>",
            "containerlab": "srl02"
          }
        }
      },
      "links": [
        {
          "a": {
            "node": "srl1",
            "interface": "e1-1",
            "mac": "<mac address>",
            "peer": "z"
          },
          "z": {
            "node": "srl2",
            "interface": "e1-1",
            "mac": "<mac address>",
            "peer": "a"
          }
        }
      ]
    }
    ```

## SSH Config

To simplify SSH access to the nodes started by Containerlab an SSH config file is generated per each deployed lab. The config file instructs SSH clients to not warn users about the changed host keys and also sets the username to the one known by Containerlab:

```title="<code>/etc/ssh/ssh_config.d/clab-[lab-name].conf</code>"
# Containerlab SSH Config for the srl lab

Host clab-srl-srl
  User admin
  StrictHostKeyChecking=no
  UserKnownHostsFile=/dev/null
```

Now you can SSH to the nodes without being prompted to accept the host key and even omitting the username.

```srl
❯ ssh clab-srl-srl
Warning: Permanently added 'clab-srl-srl' (ED25519) to the list of known hosts.
................................................................
:                  Welcome to Nokia SR Linux!                  :
:              Open Network OS for the NetOps era.             :
:                                                              :
:    This is a freely distributed official container image.    :
:                      Use it - Share it                       :
:                                                              :
: Get started: https://learn.srlinux.dev                       :
: Container:   https://go.srlinux.dev/container-image          :
: Docs:        https://doc.srlinux.dev/23-7                    :
: Rel. notes:  https://doc.srlinux.dev/rn23-7-1                :
: YANG:        https://yang.srlinux.dev/release/v23.7.1        :
: Discord:     https://go.srlinux.dev/discord                  :
: Contact:     https://go.srlinux.dev/contact-sales            :
................................................................

Using configuration file(s): []
Welcome to the srlinux CLI.
Type 'help' (and press <ENTER>) if you need any help using this.
--{ running }--[  ]--
A:srl#
```

[^1]: For example [Ansible Docker connection](https://docs.ansible.com/ansible/latest/collections/community/docker/docker_connection.html) plugin.
