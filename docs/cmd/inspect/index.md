# Inspect command

### Description

The `inspect` command provides the information about the deployed labs.

### Usage

`containerlab [global-flags] inspect [local-flags]`

### Flags

#### all

With the local `--all` flag it's possible to list all deployed labs in a single table. The output will also show the relative path to the topology file that was used to spawn this lab.

The lab name and path values will be set for the first node of such lab, to reduce the clutter. Refer to the [examples](#examples) section for more details.

#### topology | name

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

#### format

The local `--format` flag enables different output stylings. By default the table view will be used.

Available format options are:

- `json` - outputs the inspect data in JSON format
- `csv` - outputs the inspect data as comma-separated values.

The data is provided in the context of a container that belongs to a containerlab lab. Both output formats will provide you with the following data:

| Field          | Description                                                                                              |
|----------------|----------------------------------------------------------------------------------------------------------|
| `lab_name`     | The name of the lab.                                                                                     |
| `labPath`      | The relative path to the lab topology file.                                                              |
| `absLabPath`   | The absolute path to the lab topology file.                                                              |
| `name`         | The name of the container.                                                                               |
| `container_id` | The unique ID of the container.                                                                          |
| `image`        | The container's [image](../../manual/nodes.md#image).                                                    |
| `kind`         | The container's [kind](../../manual/nodes.md#kind).                                                      |
| `state`        | The container's state. ie. `created`, `restarting`, `running`, `removing`, `paused`, `exited`, or `dead` |
| `status`       | The health status of the container. ie `starting`, `unhealthy`, `up` or `exited`.                        |
| `ipv4_address` | The [IPv4 management address](../../manual/nodes.md#mgmt-ipv4) of the container.                            |
| `ipv6_address` | The [IPv6 management address](../../manual/nodes.md#mgmt-ipv6) of the container.                            |
| `owner`        | The [owner](../deploy.md#owner) of the lab.                                                                 |

#### details

The `inspect` command produces a brief summary about the running lab components. It is also possible to get a full view on the running containers by adding `--details` flag.

With this flag inspect command will output every bit of information about the running containers. This is what `docker inspect` command provides.

#### wide

The local `-w | --wide` flag adds all available columns to the `inspect` output table.

### Examples

#### List all running labs on the host

```bash
❯ containerlab inspect --all
+---+------------+----------+-----------------+--------------+--------------------+------+-------+---------+----------------+----------------------+
| # | Topo Path  | Lab Name |      Name       | Container ID |       Image        | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+------------+----------+-----------------+--------------+--------------------+------+-------+---------+----------------+----------------------+
| 1 | newlab.yml | newlab   | clab-newlab-n1  | 3c8262034088 | srlinux:20.6.3-145 | srl  |       | running | 172.20.20.4/24 | 3fff:172:20:20::4/80 |
| 2 |            |          | clab-newlab-n2  | 79c562b71997 | srlinux:20.6.3-145 | srl  |       | running | 172.20.20.5/24 | 3fff:172:20:20::5/80 |
| 3 | srl02.yml  | srl01    | clab-srl01-srl  | 13c9e7543771 | srlinux:20.6.3-145 | srl  |       | running | 172.20.20.2/24 | 3fff:172:20:20::2/80 |
| 4 |            |          | clab-srl01-srl2 | 8cfca93b7b6f | srlinux:20.6.3-145 | srl  |       | running | 172.20.20.3/24 | 3fff:172:20:20::3/80 |
+---+------------+----------+-----------------+--------------+--------------------+------+-------+---------+----------------+----------------------+
```

#### Provide information about a specific running lab by its name

Provide information about the running lab named `srlceos01`

```bash
❯ containerlab inspect --name srlceos01
+---+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+
| # |        Name         | Container ID |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+
| 1 | clab-srlceos01-ceos | 90bebb1e2c5f | ceos    | ceos |       | running | 172.20.20.4/24 | 3fff:172:20:20::4/80 |
| 2 | clab-srlceos01-srl  | 82e9aa3c7e6b | srlinux | srl  |       | running | 172.20.20.3/24 | 3fff:172:20:20::3/80 |
+---+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+
```

#### Provide information about a specific running lab by its topology file

```bash
❯ clab inspect -t srl02.clab.yml
INFO[0000] Parsing & checking topology file: srl02.clab.yml
+---+-----------------+--------------+-----------------------+------+---------+----------------+----------------------+
| # |      Name       | Container ID |         Image         | Kind |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------------+--------------+-----------------------+------+---------+----------------+----------------------+
| 1 | clab-srl02-srl1 | 7a7c101be7d8 | ghcr.io/nokia/srlinux | srl  | running | 172.20.20.4/24 | 3fff:172:20:20::4/64 |
| 2 | clab-srl02-srl2 | 5e3737621753 | ghcr.io/nokia/srlinux | srl  | running | 172.20.20.5/24 | 3fff:172:20:20::5/64 |
+---+-----------------+--------------+-----------------------+------+---------+----------------+----------------------+
```

#### Provide owner information of running labs

An owner is a linux user that started the lab. When `sudo` is used, the original user is displayed as the owner.

```bash
clab inspect --all --wide
+---+-----------------------------------+----------+-------+-----------------+--------------+-----------------------+---------------+---------+----------------+----------------------+
| # |             Topo Path             | Lab Name | Owner |      Name       | Container ID |         Image         |     Kind      |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------------------------------+----------+-------+-----------------+--------------+-----------------------+---------------+---------+----------------+----------------------+
| 1 | lab-examples/srl01/srl01.clab.yml | srl01    | user1 | clab-srl01-srl  | ea86f40b412a | ghcr.io/nokia/srlinux | nokia_srlinux | running | 172.20.20.2/24 | 3fff:172:20:20::2/64 |
| 2 | lab-examples/srl02/srl02.clab.yml | srl02    | user2 | clab-srl02-srl1 | ba7e807235b6 | ghcr.io/nokia/srlinux | nokia_srlinux | running | 172.20.20.4/24 | 3fff:172:20:20::4/64 |
| 3 |                                   |          |       | clab-srl02-srl2 | 71006155b70a | ghcr.io/nokia/srlinux | nokia_srlinux | running | 172.20.20.3/24 | 3fff:172:20:20::3/64 |
+---+-----------------------------------+----------+-------+-----------------+--------------+-----------------------+---------------+---------+----------------+----------------------+
```

#### Provide information about a specific running lab in json format

```bash
❯ containerlab inspect --name vlan -f json
{
  "vlan": [
    {
      "lab_name": "vlan",
      "labPath": "../../srlinux-vlan-handling-lab/vlan.clab.yml",
      "absLabPath": "/root/projects/srlinux-vlan-handling-lab/vlan.clab.yml",
      "name": "clab-vlan-client1",
      "container_id": "2c70173f3f41",
      "image": "ghcr.io/srl-labs/alpine",
      "kind": "linux",
      "state": "running",
      "status": "Up 4 hours",
      "ipv4_address": "172.20.20.4/24",
      "ipv6_address": "3fff:172:20:20::4/64",
      "owner": "root"
    },
    {
      "lab_name": "vlan",
      "labPath": "../../srlinux-vlan-handling-lab/vlan.clab.yml",
      "absLabPath": "/root/projects/srlinux-vlan-handling-lab/vlan.clab.yml",
      "name": "clab-vlan-client2",
      "container_id": "9a12e9566d8c",
      "image": "ghcr.io/srl-labs/alpine",
      "kind": "linux",
      "state": "running",
      "status": "Up 4 hours",
      "ipv4_address": "172.20.20.2/24",
      "ipv6_address": "3fff:172:20:20::2/64",
      "owner": "root"
    }
  ]
}
```

#### Provide information about a specific running lab in csv format

```bash
❯ containerlab inspect --name vlan -f csv
lab_name,labPath,absLabPath,name,container_id,image,kind,state,status,ipv4_address,ipv6_address,owner
vlan,srlinux-vlan-handling-lab/vlan.clab.yml,/root/srlinux-vlan-handling-lab/vlan.clab.yml,clab-vlan-client1,4a60ed6861ec,ghcr.io/srl-labs/alpine,linux,running,Up 47 seconds,172.20.20.7/24,3fff:172:20:20::7/64,root
vlan,srlinux-vlan-handling-lab/vlan.clab.yml,/root/srlinux-vlan-handling-lab/vlan.clab.yml,clab-vlan-client2,77b34781d62e,ghcr.io/srl-labs/alpine,linux,running,Up 47 seconds,172.20.20.8/24,3fff:172:20:20::8/64,root
vlan,srlinux-vlan-handling-lab/vlan.clab.yml,/root/srlinux-vlan-handling-lab/vlan.clab.yml,clab-vlan-srl1,2cedee35c8a2,ghcr.io/nokia/srlinux:24.10.1,nokia_srlinux,running,Up 47 seconds,172.20.20.6/24,3fff:172:20:20::6/64,root
vlan,srlinux-vlan-handling-lab/vlan.clab.yml,/root/srlinux-vlan-handling-lab/vlan.clab.yml,clab-vlan-srl2,c8c66491e10c,ghcr.io/nokia/srlinux:24.10.1,nokia_srlinux,running,Up 47 seconds,172.20.20.9/24,3fff:172:20:20::9/64,root
```
