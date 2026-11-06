# capture command

### Description

The `capture` command captures packets on lab link interfaces of one or more nodes and writes them as one [pcapng](https://www.ietf.org/archive/id/draft-ietf-opsawg-pcapng-03.html) stream, to a file or to stdout for Wireshark.

It works with every runtime:

- **Docker and Podman**: containerlab opens a packet socket on each interface inside the node's network namespace. Nothing runs in the node, the node image needs no capture tools, and no extra software is needed on the host: the command needs root, like `deploy`. See [How Docker and Podman captures work](#how-docker-and-podman-captures-work).
- **c9s**: the `clabwire` sidecar of each node's pod captures the packets; see the [c9s runtime](../manual/clabernetes/runtime.md#packet-capture).

The output is the same with both. [Edgeshark](../manual/wireshark.md) remains an alternative for Docker labs when you prefer to pick interfaces in a GUI.

--8<-- "docs/cmd/deploy.md:env-vars-flags"

### Usage

`containerlab [global-flags] capture [local-flags]`

### Flags

#### topology | name

Use the global `--topo | -t` flag to reference the lab topology file, or use the global `--name` flag to reference an already deployed lab by name. Without either, containerlab looks for a `*.clab.yml` file in the current directory, like the other lab commands.

With Docker and Podman, the capturable interfaces are the links of the topology file, so `--name` needs the topology file the lab was deployed from to still be in place.

#### target

The local `--target` flag selects what to capture, as `<node>:<interface>[,<interface>...]`. Repeat it for more nodes:

```text
--target srl1:e1-1,e1-2 --target srl2:e1-1
```

The node name ends at the first colon, so interface names such as `1/1/3` or `ethernet-1/1` need no quoting. Node names are accepted as the topology file spells them; with Docker and Podman also as the container name (`clab-srl-srl1`), with c9s also as they were sanitized for Kubernetes (`R1` or `r1`). Interfaces are accepted by their name in the node or by the alias the topology used for them (`e1-1` or `ethernet-1/1` for SR Linux). `--list` shows both.

Only lab link interfaces can be captured; the management interface cannot. An interface that is not a link interface of the node fails the command before anything is captured, and the error lists the node's capturable interfaces.

#### duration

The local `--duration` flag stops the capture after the given time (`30s`, `5m`, `2h`). The default `0` captures until the command is interrupted with Ctrl+C or SIGTERM. Captures may run longer than an hour; see [Long captures](#long-captures).

#### packet-limit

The local `--packet-limit` flag stops the capture after that many packets, counted across all targets. `0`, the default, means no limit.

#### snap-length

The local `--snap-length` flag sets how many bytes of each packet are kept, from 64 to 1048576. The default `0` keeps 256 KiB.

#### write

The local `--write | -w` flag names the output file. The default `-` writes to stdout.

#### list

The local `--list` flag lists the interfaces `capture` accepts, for every running node, with the node and interface at the other end of each link, instead of capturing. `--format | -f` selects `table` (default) or `json`.

### Output

The output is one pcapng section:

- a Section Header Block with `shb_userappl` set to `containerlab`
- one Interface Description Block per target, in the order of the `--target` flags, written before any packet: link type Ethernet, the snap length, nanosecond timestamps, and the name `<node>:<interface>` (for example `srl1:e1-1`), with the node named as the topology spells it
- Enhanced Packet Blocks of all targets, in timestamp order: packets are held briefly (100 ms with Docker and Podman, 250 ms with c9s) and written ordered by timestamp, so packets of different interfaces come out in order even though each one is read on its own; a packet that arrives later than that is written as it comes

The interface name shows in Wireshark's interface column (`frame.interface_name`), so a capture of several nodes can be filtered by node and interface.

When the capture ends (duration, packet limit, Ctrl+C or SIGTERM), the command finishes the stream and exits with code 0; the file is a valid capture. Status messages go to stderr. With c9s, every `clabwire` capture in the pods ends with the command: containerlab closes its stdin, and `clabwire` (`--stop-on-stdin-close`) stops.

`--list -f json` prints an array:

```json
[
  {
    "node": "srl1",
    "interface": "e1-1",
    "alias": "ethernet-1/1",
    "peer_node": "srl2",
    "peer_interface": "ethernet-1/1"
  }
]
```

`alias` is present only when the topology used an alias for the interface. `peer_node` is `host` for a host endpoint and `mgmt-net` for a management network endpoint.

### How Docker and Podman captures work

For each target, containerlab enters the node's network namespace, opens a Linux packet socket (`AF_PACKET`) bound to the interface, and leaves the namespace again; the socket keeps capturing on that interface. This is what `tcpdump` does inside the node, without needing `tcpdump` in the node or on the host. All sockets are opened before the first byte is written, so a capture that cannot start (a node that does not run, a missing interface) fails without output.

- Packets carry the kernel's receive timestamps, so packets of different nodes compare on one clock.
- 802.1Q tags, which the kernel keeps outside the frame, are put back in place, so VLAN traffic shows its tags as on the wire.
- Each socket is in promiscuous mode for as long as it is open; the interface returns to its previous state when the capture ends.
- An interface that goes down and up again keeps being captured. When an interface disappears, for example while its node restarts, containerlab waits for it and reattaches once the node is back (`containerlab restart --node`). The time it was gone is reported as a warning with its start and end time.

The command needs root (to open packet sockets and enter network namespaces), as other lab commands do.

### Long captures

Docker and Podman captures have no length limit. With c9s, one `clabwire` capture lasts at most one hour. `capture` chains captures: it starts the next one shortly before the current one ends, lets both run for a few seconds, and switches between them in the middle of that overlap. The repeated section and interface headers of the follow-up are dropped, and packets seen by both captures near the switch are written once (they are matched by content, because each capture stamps packets when it reads them). Once the switch time has passed, the previous capture is stopped through its stdin instead of running to its end. If a follow-up capture does not start in time, the missing interval is reported as a warning with its start and end time; it is never skipped silently. The timestamp order holds across switches: while one node's capture switches, packets of other nodes stamped after the switch wait for it.

### Examples

#### Capture two nodes for 30 seconds into one file

```bash
containerlab capture -t srl.clab.yml \
  --target srl1:e1-1 --target srl2:e1-1 --duration 30s -w srl.pcapng
```

#### Watch a capture live in Wireshark

```bash
containerlab capture --name srl --target srl1:e1-1,e1-2 -w - | wireshark -k -i -
```

#### Capture the first 100 packets with tshark

```bash
containerlab capture --name srl --target srl1:ethernet-1/1 --packet-limit 100 -w - | tshark -r -
```

#### List the capturable interfaces

```bash
containerlab capture --name srl --list
```

#### Capture a c9s lab

```bash
containerlab -r c9s capture --name srl --target srl1:e1-1 --target srl2:e1-1 -w srl.pcapng
```
