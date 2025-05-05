# sshx list

## Description

The `list` sub-command under the `tools sshx` command displays all active SSHX containers across all labs. This command provides a comprehensive view of all running terminal sharing sessions, including their network association, status, IP address, sharing links, and owner information.

This is useful for:

- Identifying all active sharing sessions
- Getting the sharing links for existing sessions
- Seeing who created each sharing session
- Checking the status of SSHX containers

## Usage

```
containerlab tools sshx list [flags]
```

## Flags

### --format | -f

The output format for the list, specified with `--format | -f` flag. Possible values:

- `table` (default) - Displays the information in a formatted table
- `json` - Outputs the information in JSON format

The JSON output is particularly useful for scripting or programmatic access to the list of SSHX containers.

## Examples

```bash
# List all active SSHX containers in table format (default)
❯ containerlab tools sshx list
┌───────────┬────────────┬─────────┬──────────────┬─────────────────────────────────┬───────────┐
│ NAME      │ NETWORK    │ STATUS  │ IPv4 ADDRESS │ LINK                            │ OWNER     │
├───────────┼────────────┼─────────┼──────────────┼─────────────────────────────────┼───────────┤
│ sshx-lab1 │ clab-lab1  │ running │ 172.20.20.5  │ https://sshx.io/s#sessionid,key │ alice     │
├───────────┼────────────┼─────────┼──────────────┼─────────────────────────────────┼───────────┤
│ sshx-lab2 │ clab-lab2  │ running │ 172.20.30.8  │ https://sshx.io/s#sessionid,key │ bob       │
└───────────┴────────────┴─────────┴──────────────┴─────────────────────────────────┴───────────┘

# List all SSHX containers in JSON format
❯ containerlab tools sshx list -f json
[
  {
    "name": "clab-lab1-sshx",
    "network": "clab-lab1",
    "state": "running",
    "ipv4_address": "172.20.20.5",
    "link": "https://sshx.io/s#sessionid,accesskey",
    "owner": "alice"
  },
  {
    "name": "clab-lab2-sshx",
    "network": "clab-lab2",
    "state": "running",
    "ipv4_address": "172.20.30.8",
    "link": "https://sshx.io/s#sessionid,accesskey",
    "owner": "bob"
  }
]

# When no active SSHX containers exist
❯ containerlab tools sshx list
No active SSHX containers found
```

The list command shows the following information for each SSHX container:

- **NAME**: The name of the SSHX container
- **NETWORK**: The network the container is attached to
- **STATUS**: The current status of the container (running, stopped, etc.)
- **IPv4 ADDRESS**: The IP address assigned to the container on the network
- **LINK**: The shareable SSHX terminal link (displays "N/A" if the link isn't ready or there's an error)
- **OWNER**: The person who created the SSHX container (from the --owner flag or environment variables)
