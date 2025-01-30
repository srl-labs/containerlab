---
---

# Containerlab VS Code Extension

The lab-as-code approach taken by Containerlab means labs are written in YAML. This means you need a text editor to write and modify your labs. It also means you have to manage your labs via the command-line.

VS Code is a fine text editor used by many for this purpose, and with the YAML schema provided by Containerlab the topology writing experience is made even easier. 

We decided to further improve the experience with VS Code with a Containerlab [VS Code extension](https://marketplace.visualstudio.com/items?itemName=srl-labs.vscode-containerlab).

The Containerlab VS Code extension aims to greatly simplify and improve the labbing workflow, allowing you do do everything you need to lab from directly in VS Code.

## Features

The extension is feature rich while providing flexibility way you can interact with such features. This is all discussed below.

### Explorer

In the activity bar of VS Code, you will notice a Containerlab icon. Clicking on this icon will open the explorer.

The explorer is similar to the VS Code file explorer but instead of finding files, it discovers Containerlab topologies, running labs, containers and their interfaces.

The explorer will discover all labs in your local directory (and subdirectories), as well as any running labs on the system.

The explorer is a Treeview and for running labs, you can expand the labs to see running containers and you can expand running containers to see their interfaces.

** Video of explorer interaction **

### Editor

In the editor title actions when a Containerlab topology is open, there is a 'run' action and a 'graph' button.

This allows easy deployment and graphing of the lab from within the editor context.

** Some picture/video of editor action interaction **

#### Command palette

When you have a topology open and active in the editor you can execute common actions from the command palette.

** Command palette picture/video **

#### Keybindings

We have also set some default keybindings you can use to interact with the lab when you are editing a topology.

| Keys         | Action   |
| ------------ | -------- |
| `CTRL+ALT+D` | Deploy   |
| `CTRL+ALT+R` | Redeploy |
| `CTRL+ALT+K` | Destroy  |
| `CTRL+ALT+G` | Graph    |

### TopoViewer

Integrated into the extension as a 'graph' action is the [TopoViewer](https://github.com/asadarafat/topoViewer) project by @asadarafat.

TopoViewer is an interactive way to visualize your containerlab topologies.

** Topoviewer video/picture **

### Packet capture

In the explorer you can expand running labs and containers to view all the discovered interfaces for that container. You can either right click on the interface and start a packet capture or click the shark icon on the interface label.

## Settings reference

Below is a reference to the available settings that can be modified in the Containerlab VS Code extension.

### `containerlab.defaultSshUser`

The default username used to connect to a node via SSH.

| Type     | Default |
| -------- | ------- |
| `string` | `admin` |

### `containerlab.sudoEnabledByDefault`

Whether or not to append `sudo` to any commands executed by the extension.

| Type      | Default |
| --------- | ------- |
| `boolean` | `true`  |

### `containerlab.refreshInterval`

The time interval (in milliseonds) for which the extension automatically refreshes.

On each refresh the extension will discover all local and running labs, as well as containers and interfaces of containers belonging to running labs.

By default this is 10 seconds.

| Type     | Default |
| -------- | ------- |
| `number` | `10000` |

### `containerlab.node.execCommandMapping`

The a mapping between the node kind and command executed on the 'node attach' action.

The 'node attach' action performs the `docker exec -it <node> <command>` command. The `execCommandMapping` allows you to change the command executed by default on a per-kind basis.

By default this setting is empty, it should be used to override the [default mappings](https://github.com/srl-labs/vscode-containerlab/blob/main/resources/exec_cmd.json).

| Type     | Default     |
| -------- | ----------- |
| `object` | `undefined` |

#### Example
```json
{
    "nokia_srl": "sr_cli",
}
```

In the settings UI, simply set the 'Item' field to the kind and the 'Value' field to `nokia_srl` and the command to `sr_cli`.