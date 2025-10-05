# code-server status

## Description

The `status` sub-command under the `tools code-server` command inspects the active code-server containers that were launched via `containerlab`. It reports each container's runtime state, exposed port, mounted labs directory, and owner label so that you can quickly find connection details or verify cleanup.

## Usage

```
containerlab tools code-server status [flags]
```

## Flags

### --format | -f

Output format for the listing. Accepts `table` (default) or `json`.

## Examples

List all running code-server containers in table form:

```bash
❯ containerlab tools code-server status
╭──────────────────┬─────────┬───────┬──────────┬───────╮
│ NAME             │ STATUS  │  PORT │ LABS DIR │ OWNER │
├──────────────────┼─────────┼───────┼──────────┼───────┤
│ clab-code-server │ running │ 32779 │ ~/.clab  │ clab  │
╰──────────────────┴─────────┴───────┴──────────┴───────╯
```

Show the same information in JSON format (useful for scripting):

```bash
❯ containerlab tools code-server status --format json
[
  {
    "name": "clab-code-server",
    "state": "running",
    "host": "",
    "port": 32779,
    "labs_dir": "~/.clab",
    "owner": "clab"
  }
]
```

When no containers are active the command prints `No active code-server containers found` (or an empty JSON array when `--format json` is used).
