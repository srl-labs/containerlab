# Setting link impairments

With the `containerlab tools netem set` command users can set link impairments on a specific interface of a container. The following list of link impairments is supported:

* delay & jitter
* packet loss
* rate limiting

/// details | Considerations
Note, that `netem` is a Linux kernel module and it might not be available in particular kernel configurations.

For example, on some RHEL 8 systems the following commands might be needed to run to add the support for netem:

```bash
dnf install kernel-debug-modules-extra
dnf install kernel-modules-extra
systemctl reboot now 
```

///

Note, that setting link impairments with `netem set` command is implemented in a way that all impairments are applied to the interface at once. This means that if an interface had a packet loss of 10% and you execute `netem set` command with a delay of 100ms, the packet loss will be reset to 0% and the delay will be set to 100ms.

Once the impairments are set, they act for as long as the underlying node/container is running. To clear the impairments, set them to the [default values](#clear-any-existing-impairments).

## Usage

```bash
containerlab tools netem set [local-flags]
```

## Flags

### node

With the mandatory `--node | -n` flag a user specifies the name of the containerlab node to set link impairments on.

### interface

With the mandatory `--interface | -i` flag a user specifies the name of the interface to set link impairments on. This can also be the [interface alias](../../../manual/topo-def-file.md#interface-naming), if one is used.

### delay

With the `--delay` flag a user specifies the delay to set on the interface. The delay is specified in duration format. Example: `50ms`, `3s`.

Default value is `0s`.

### jitter

Delay variation, aka jitter, is specified with the `--jitter` flag. The jitter is specified in duration format and can only be used if `--delay` is specified. Example: `5ms`.

Default value is `0s`.

### loss

Packet loss is specified with the `--loss` flag. The loss is specified in percentage format. Example: `10`.

### rate

Egress rate limiting is specified with the `--rate` flag. The rate is specified in kbit per second format. Example: value `100` means rate of 100kbit/s.

## Examples

### Setting delay and jitter

For `clab-netem-r1` node and its `eth1` interface set delay of 5ms and jitter of 1ms:

```bash
containerlab tools netem set -n clab-netem-r1 -i eth1 --delay 5ms --jitter 1ms
```

### Setting packet loss

```bash title="setting packet loss at 10% rate"
containerlab tools netem set -n clab-netem-r1 -i eth1 --loss 10
```

### Clear any existing impairments

```bash
containerlab tools netem set -n clab-netem-r1 -i eth1
+-----------+-------+--------+-------------+-------------+
| Interface | Delay | Jitter | Packet Loss | Rate (kbit) |
+-----------+-------+--------+-------------+-------------+
| eth1      | 0s    | 0s     | 0.00%       |           0 |
+-----------+-------+--------+-------------+-------------+
```

The above command will use default values for all supported link impairments, which is `0s` for delay and jitter, `0` for loss and `0` for rate.
