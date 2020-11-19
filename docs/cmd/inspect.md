# inspect command

### Description

The `inspect` command provides the information about the deployed labs.

### Usage

`containerlab [global-flags] inspect [local-flags]`

### Flags

#### all
With the local `--all` flag its possible to list all deployed labs in a single table.

#### topology | name

With the global `--topo | -t` or `--name | -n` flag a user specifies which particular lab they want to get the information about.

#### format

The local `--format` flag enables different output stylings. By default the table view will be used.

Currently, the only other format option is `json` that will produce the output in the JSON format.

#### details
The `inspect` command produces a brief summary about the running lab components. It is also possible to get a full view on the running containers by adding `--details` flag.

With this flag inspect command will output every bit of information about the running containers. This is what `docker inspect` command provides.

### Examples

```bash
# list all running labs on the host
containerlab inspect --all
+---+-----------+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+
| # | Lab Name  |        Name         | Container ID |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+
| 1 | srl01     | clab-srl01-srl      | 37156faa5444 | srlinux | srl  |       | running | 172.20.20.2/24 | 2001:172:20:20::2/80 |
| 2 | srlceos01 | clab-srlceos01-ceos | 90bebb1e2c5f | ceos    | ceos |       | running | 172.20.20.4/24 | 2001:172:20:20::4/80 |
| 3 | srlceos01 | clab-srlceos01-srl  | 82e9aa3c7e6b | srlinux | srl  |       | running | 172.20.20.3/24 | 2001:172:20:20::3/80 |
+---+-----------+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+

# provide information about the running lab named srl02
containerlab inspect --name srlceos01
+---+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+
| # |        Name         | Container ID |  Image  | Kind | Group |  State  |  IPv4 Address  |     IPv6 Address     |
+---+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+
| 1 | clab-srlceos01-ceos | 90bebb1e2c5f | ceos    | ceos |       | running | 172.20.20.4/24 | 2001:172:20:20::4/80 |
| 2 | clab-srlceos01-srl  | 82e9aa3c7e6b | srlinux | srl  |       | running | 172.20.20.3/24 | 2001:172:20:20::3/80 |
+---+---------------------+--------------+---------+------+-------+---------+----------------+----------------------+


# now in json format
containerlab inspect --name srlceos01 -f json
[
  {
    "lab_name": "srlceos01",
    "name": "clab-srlceos01-srl",
    "container_id": "82e9aa3c7e6b",
    "image": "srlinux",
    "kind": "srl",
    "state": "running",
    "ipv4_address": "172.20.20.3/24",
    "ipv6_address": "2001:172:20:20::3/80"
  },
  {
    "lab_name": "srlceos01",
    "name": "clab-srlceos01-ceos",
    "container_id": "90bebb1e2c5f",
    "image": "ceos",
    "kind": "ceos",
    "state": "running",
    "ipv4_address": "172.20.20.4/24",
    "ipv6_address": "2001:172:20:20::4/80"
  }
]
```