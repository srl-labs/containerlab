# exec command

## Description

The `exec` command allows a user to execute a command inside the nodes (containers).

This command is similar to `docker exec`, but it allows a user to run the same command across multiple lab nodes matching the filter. Users can provide a path to the topology file and use the `--label` argument to narrow down the list of nodes to execute the command on.

## Usage

`containerlab [global-flags] exec [local-flags]`

## Flags

### topology

With the global `--topo | -t` flag a user can set a path to the topology file that will be used to filter the nodes targeted for execution of the command. The nodes can further be filtered with the `--label` flag.

Note, that with the nodes of [`ext-container` type](../manual/kinds/ext-container.md), the topology must not be provided.

### cmd

The command to be executed on the nodes is provided with `--cmd` flag. The command is provided as a string, thus it needs to be quoted to accommodate for spaces or special characters.

### format

The `--format | -f` flag allows selecting between plain text format output or a json variant. Consult with the examples below to see the differences between these two formatting options.

Defaults to `plain` output format.

### label

Using `--label` it is possible to filter the nodes to execute the command on using labels attached to the nodes. The label is provided as a string in the form of `key=value`. The `key` is the label name, and the `value` is the label value.

Exec command should either be provided with a topology file, or labels, or both.

Recall that you can check the labels attached to the nodes with `docker inspect -f '{{.Config.Labels | json}}' <container-name>` command.

## Examples

### Execute a command on all nodes of the lab

Show ipv4 information from all the nodes of the lab defined in `srl02.clab.yml` with a plain text output

```bash
❯ containerlab exec -t srl02.clab.yml --cmd 'ip -4 a show dummy-mgmt0'
INFO[0000] clab-srl02-srl1: stdout:
6: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    inet 172.20.20.3/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever
INFO[0000] clab-srl02-srl2: stdout:
6: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    inet 172.20.20.2/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever
```

### Execute a command on a node referenced by its name

Show ipv4 information from a specific node of the lab with a plain text output

```bash
❯ containerlab exec -t srl02.clab.yml --label clab-node-name=srl2 --cmd 'ip -4 a show dummy-mgmt0'
INFO[0000] Parsing & checking topology file: srl02.yml  
INFO[0000] Executed command 'ip -4 a show dummy-mgmt0' on clab-srl02-srl2. stdout:
6: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    inet 172.20.20.5/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever 
```

### Execute a command on multiple nodes referenced by a filter and no topology file

Since containerlab injects default labels to the nodes, it is possible to leverage `clab-node-kind` label that is attached to all the nodes. This label contains the node kind (type) information. In this example we will execute a command on all the nodes of the lab that are of `nokia_srlinux` kind.

```bash
❯ sudo clab exec --label clab-node-kind=nokia_srlinux --cmd "ip -4 addr show dummy-mgmt0"
INFO[0000] Executed command "ip -4 addr show dummy-mgmt0" on the node "greeter-srl". stdout:
2: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    inet 172.20.20.2/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever 
INFO[0000] Executed command "ip -4 addr show dummy-mgmt0" on the node "srl". stdout:
2: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    inet 172.20.20.3/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever 
```

### Execute a CLI Command

```bash
❯ containerlab exec -t srl02.yml --cmd 'sr_cli  "show version"'
INFO[0001] clab-srl02-srl1: stdout:
----------------------------------------------------
Hostname          : srl1
Chassis Type      : 7250 IXR-6
Part Number       : Sim Part No.
Serial Number     : Sim Serial No.
System MAC Address: 02:00:6B:FF:00:00
Software Version  : v20.6.3
Build Number      : 145-g93496a3f8c
Architecture      : x86_64
Last Booted       : 2021-06-24T10:25:26.722Z
Total Memory      : 24052875 kB
Free Memory       : 21911906 kB
----------------------------------------------------
INFO[0003] clab-srl02-srl2: stdout:
----------------------------------------------------
Hostname          : srl2
Chassis Type      : 7250 IXR-6
Part Number       : Sim Part No.
Serial Number     : Sim Serial No.
System MAC Address: 02:D8:A9:FF:00:00
Software Version  : v20.6.3
Build Number      : 145-g93496a3f8c
Architecture      : x86_64
Last Booted       : 2021-06-24T10:25:26.904Z
Total Memory      : 24052875 kB
Free Memory       : 21911914 kB
----------------------------------------------------
```

### Execute a Command with json formatted output

```bash
❯ containerlab exec -t srl02.yml --cmd 'sr_cli  "show version | as json"' -f json | jq
```

```json
{
  "clab-srl02-srl1": {
    "stderr": "",
    "stdout": {
      "basic system info": {
        "Architecture": "x86_64",
        "Build Number": "145-g93496a3f8c",
        "Chassis Type": "7250 IXR-6",
        "Free Memory": "21911367 kB",
        "Hostname": "srl1",
        "Last Booted": "2021-06-24T10:25:26.722Z",
        "Part Number": "Sim Part No.",
        "Serial Number": "Sim Serial No.",
        "Software Version": "v20.6.3",
        "System MAC Address": "02:00:6B:FF:00:00",
        "Total Memory": "24052875 kB"
      }
    }
  },
  "clab-srl02-srl2": {
    "stderr": "",
    "stdout": {
      "basic system info": {
        "Architecture": "x86_64",
        "Build Number": "145-g93496a3f8c",
        "Chassis Type": "7250 IXR-6",
        "Free Memory": "21911367 kB",
        "Hostname": "srl2",
        "Last Booted": "2021-06-24T10:25:26.904Z",
        "Part Number": "Sim Part No.",
        "Serial Number": "Sim Serial No.",
        "Software Version": "v20.6.3",
        "System MAC Address": "02:D8:A9:FF:00:00",
        "Total Memory": "24052875 kB"
      }
    }
  }
}
```
