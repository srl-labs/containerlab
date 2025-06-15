# gotty list

## Description

The `list` sub-command under the `tools gotty` command shows all active GoTTY containers. Information such as container name, network, state, IP address, port, web URL and owner are presented.

## Usage

```
containerlab tools gotty list [flags]
```

## Flags

### `--format | -f`

Output format for the command. Either `table` (default) or `json`.

## Examples

List GoTTY containers in table format:

```bash
❯ containerlab tools gotty list
NAME              NETWORK     STATUS   IPv4 ADDRESS   PORT   WEB URL                OWNER
clab-mylab-gotty  clab-mylab  running  172.20.20.5    8080   http://HOST_IP:8080    alice
```

List GoTTY containers in JSON format:

```bash
❯ containerlab tools gotty list -f json
[
  {
    "name": "clab-mylab-gotty",
    "network": "clab-mylab",
    "state": "running",
    "ipv4_address": "172.20.20.5",
    "port": 8080,
    "web_url": "http://HOST_IP:8080",
    "owner": "alice"
  }
]
```

If no containers are running the command prints `No active GoTTY containers found` (or `[]` in JSON mode).
