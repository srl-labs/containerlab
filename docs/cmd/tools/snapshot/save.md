# snapshot save

## Description

The `tools snapshot save` command creates snapshots of running vrnetlab-based VMs and saves them to disk. Snapshots capture the complete VM state including configuration, running processes, and memory, allowing for fast restoration later.

Each node's snapshot is saved as `{output-dir}/{nodename}.tar`.

Only vrnetlab-based nodes support snapshots. Non-vrnetlab nodes are automatically skipped.

## Usage

`containerlab tools snapshot save [flags]`

## Flags

### output-dir

Specify the directory where snapshot files will be saved.

Default value is `./snapshots`.

```bash
containerlab tools snapshot save --output-dir /backups/lab1
```

Creates snapshot files as `/backups/lab1/{nodename}.tar`

### node-filter

Specify a comma-separated list of nodes to snapshot. Only the specified nodes will be processed.

```bash
containerlab tools snapshot save --node-filter r1,r2,r3
```

### timeout

Set the maximum time to wait for each node's snapshot creation.

Default value is `5m` (5 minutes).

```bash
containerlab tools snapshot save --timeout 10m
```

### format

Output format for the results summary. Possible values: `table`, `json`.

Default value is `table`.

## Examples

### Save all vrnetlab nodes

```bash
containerlab tools snapshot save -t mylab.clab.yml
```

Creates `./snapshots/r1.tar`, `./snapshots/r2.tar`, etc.

### Save specific nodes to custom directory

```bash
containerlab tools snapshot save -t mylab.clab.yml \
  --node-filter r1,r2 \
  --output-dir /backups/2024-01-15
```

### Save with extended timeout

```bash
containerlab tools snapshot save -t mylab.clab.yml --timeout 10m
```
