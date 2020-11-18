# inspect command

### Description

The `inspect` command provides the information about a running lab.

### Usage

`containerlab [global-flags] inspect [local-flags]`

### Flags

#### topology | name

With the global `--topo | -t` or `--name | -n` flag a user specifies which lab they want to get the information about.

#### format

The local `--format` flag enables different output stylings. By default the table view will be used.

Currently, the only other format option is `json` that will produce the output in the JSON format.

#### details
The `inspect` command produces a brief summary about the running lab components. It is also possible to get a full view on the running containers by adding `--details` flag.

With this flag inspect command will output every bit of information about the running containers. This is what `docker inspect` command provides.

### Examples

```bash
# provide information about the running lab named srl02
containerlab inspect --name srl02
+-----------------+---------+------+-------+---------+----------------+----------------------+
|      Name       |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+-----------------+---------+------+-------+---------+----------------+----------------------+
| clab-srl02-srl1 | srlinux | srl  |       | running | 172.20.20.3/24 | 2001:172:20:20::3/80 |
| clab-srl02-srl2 | srlinux | srl  |       | running | 172.20.20.2/24 | 2001:172:20:20::2/80 |
+-----------------+---------+------+-------+---------+----------------+----------------------+


# now in json format
containerlab inspect --name srl02 -f json
[
  {
    "name": "clab-srl02-srl1",
    "image": "srlinux",
    "kind": "srl",
    "state": "running",
    "ipv4_address": "172.20.20.3/24",
    "ipv6_address": "2001:172:20:20::3/80"
  },
  {
    "name": "clab-srl02-srl2",
    "image": "srlinux",
    "kind": "srl",
    "state": "running",
    "ipv4_address": "172.20.20.2/24",
    "ipv6_address": "2001:172:20:20::2/80"
  }
]
```