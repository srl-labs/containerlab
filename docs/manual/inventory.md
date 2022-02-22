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
=== "ansible docker host inventory file"
    ``` yaml
    all:
      children:
        linux:
          hosts:
            clab-ansible-linux-host:
              ansible_host: <mgmt-ipv4-address>
              ansible_docker_host: clab-ansible-linux-host
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

[^1]: For example [Ansible Docker connection](https://docs.ansible.com/ansible/latest/collections/community/docker/docker_connection.html) plugin.