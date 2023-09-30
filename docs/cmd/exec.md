# exec command

### Description

The `exec` command allows to run a command inside the nodes that part of a certain lab.

This command does exactly the same thing as `docker exec` does, but it allows to run the same command across all the nodes of a lab.

### Usage

`containerlab [global-flags] exec [local-flags]`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

#### cmd

The command to be executed on the nodes is provided with `--cmd` flag. The command is provided as a string, thus it needs to be quoted to accommodate for spaces or special characters.

#### format

The `--format | -f` flag allows to select between plain text format output or a json variant. Consult with the examples below to see the differences between these two formatting options.

Defaults to `plain` output format.

#### label

By default `exec` command will attempt to execute the command across all the nodes of a lab. To limit the scope of the execution, the users can leverage the `--label` flag to filter out the nodes of interest.

#### node-filter

The local `--node-filter` flag allows a user to specify a subset of nodes from the topology to exec the command(s) on, instead of all (default). Applies to executions where the topology file is provided.

### Examples

#### Execute a command on all nodes of the lab

Show ipv4 information from all the nodes of the lab with a plain text output

```bash
❯ containerlab exec -t srl02.yml --cmd 'ip -4 a show dummy-mgmt0'
INFO[0000] clab-srl02-srl1: stdout:
6: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    inet 172.20.20.3/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever
INFO[0000] clab-srl02-srl2: stdout:
6: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    inet 172.20.20.2/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever
```

#### Execute a command on a node referenced by its name

Show ipv4 information from a specific node of the lab with a plain text output

```bash
❯ containerlab exec -t srl02.yml --label clab-node-name\=srl2 --cmd 'ip -4 a show dummy-mgmt0'
INFO[0000] Parsing & checking topology file: srl02.yml  
INFO[0000] Executed command 'ip -4 a show dummy-mgmt0' on clab-srl02-srl2. stdout:
6: dummy-mgmt0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default qlen 1000
    inet 172.20.20.5/24 brd 172.20.20.255 scope global dummy-mgmt0
       valid_lft forever preferred_lft forever 
```

#### Execute a CLI Command

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

#### Execute a Command with json formatted output

```bash
❯ containerlab exec -t srl02.yml --cmd 'sr_cli  "show version | as json"' -f json | jq
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
