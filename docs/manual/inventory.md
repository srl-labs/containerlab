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