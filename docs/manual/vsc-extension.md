# Containerlab VS Code Extension

The lab-as-code approach taken by Containerlab means labs are "written" in YAML in a text editor or IDE. It also means you have to manage your labs via the command-line.

[VS Code](https://code.visualstudio.com/download) is a powerful text editor used by many for this purpose, and with the YAML schema provided by Containerlab the topology writing experience is made even easier.

We decided to further improve the experience with VS Code with a Containerlab [VS Code extension](https://marketplace.visualstudio.com/items?itemName=srl-labs.vscode-containerlab).

The Containerlab VS Code extension aims to greatly simplify and improve the labbing workflow, allowing you to perform essential lab operations within VS Code.

If you prefer to sit back and watch a video about this extension, say no more!

-{{youtube(url='https://www.youtube.com/embed/NIw1PbfCyQ4')}}-

/// admonition | Support
If you have any questions about this extension, please join us in the [Containerlab Discord](https://discord.gg/vAyddtaEV9).
///

## Features

The Containerlab extension packs a lot of features hidden under the icons or context menus. Here is a list of the main features, and we are always looking for new feature requests:

### Explorer

In the activity bar of VS Code, you will notice a Containerlab icon. Clicking on this icon will open the explorer.

The explorer is similar to the VS Code file explorer but instead of finding files, it discovers Containerlab topologies, running labs, containers and their interfaces.

The explorer will discover all labs in your local directory (and subdirectories), as well as any running labs on the system.

The explorer is a Treeview and for running labs, you can expand the labs to see running containers and you can expand running containers to see their interfaces.

### Editor

In the editor title actions when a Containerlab topology is open, there is a 'run' action and a 'graph' button.

This allows easy deployment and graphing of the lab from within the editor context.

![editor-actions](https://gitlab.com/rdodin/pics/-/wikis/uploads/c095d277e032ad0754b5f61f4a9057f2/CleanShot_2025-02-04_at_13.56.21_2x.png)

#### Command palette

When you have a topology open and active in the editor you can execute common actions from the command palette that you can open up with `CTRL+SHIFT+P`/`CMD+SHIFT+P`.

![palette](https://gitlab.com/rdodin/pics/-/wikis/uploads/be8d2c2d7d806dedd76c2b7a924d153d/CleanShot_2025-02-04_at_13.58.39_2x.png)

#### Keybindings

We have also set some default keybindings you can use to interact with the lab when you are editing a topology.

| Keys         | Action   |
| ------------ | -------- |
| `CTRL+ALT+D` | Deploy   |
| `CTRL+ALT+R` | Redeploy |
| `CTRL+ALT+K` | Destroy  |
| `CTRL+ALT+G` | Graph    |

### draw.io
Integrated as a 'graph' action within the extension, The [clab-io-draw](https://github.com/srl-labs/clab-io-draw) project unifies two tools, clab2drawio and drawio2clab. These tools facilitate the conversion between Containerlab YAML files and Draw.io diagrams, making it easier for network engineers and architects to visualize, document, and share their network topologies.

### TopoViewer

Integrated as a 'graph' action within the extension, the [TopoViewer](https://github.com/asadarafat/topoViewer) project by @asadarafat offers an interactive way to visualize your containerlab topologies. Please ensure that containerlab is running for TopoViewer to function properly.

#### Enhanced Containerlab Topology Visualization with TopoViewer

By adding specific labels to your Containerlab topology definition, you can customize device icons, logically group nodes, and even define geo-positioning for a more intuitive network diagram.

TopoViewer leverages custom labels in your Containerlab topology to:

  - **Customize Icons:** Assign specific icons to nodes based on their role.
  - **Organize Nodes:** Group nodes under defined categories with hierarchical levels.
  - **Position Nodes Geographically:** Use geo-coordinates to map node positions.


/// details | Supported TopoViewer Labels
these are supported TopoViewer labes
//// tab | graph-icon

  - **Value Format:** `string`
  - **Alias:** `topoViewer-role`
  - **Purpose:** Defines the role of each node. TopoViewer maps these roles to unique icons.
  - **Available Roles and Icons:**

       | Role           | Icon                                                                                                                                         |
        |----------------|----------------------------------------------------------------------------------------------------------------------------------------------|
        | **pe** / **router**         | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st1{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}</style><rect width="50" height="50" class="st0"/><g><g><path d="M29.9,8.2V20h11.67" class="st1"/><path d="M38,16.04l3.12,3.17c0.54,0.54,0.54,1.29,0,1.79L38,24.17" class="st1"/></g><g><path d="M8.33,19.92h11.83v-11.67" class="st1"/><path d="M16.17,11.79l3.17,-3.12c0.54,-0.54,1.29,-0.54,1.79,0l3.21,3.17" class="st1"/></g><g><path d="M20,41.79V30H8.33" class="st1"/><path d="M11.88,33.96L8.75,30.79c-0.54,-0.54,-0.54,-1.29,0,-1.79l3.17,-3.21" class="st1"/></g><g><path d="M41.67,29.96H29.83v11.67" class="st1"/><path d="M33.83,38.08l-3.17,3.12c-0.54,0.54,-1.29,0.54,-1.79,0l-3.21,-3.17" class="st1"/></g></g></svg> |
        | **dcgw**       | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st1{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}</style><rect width="50" height="50" class="st0"/><g><g><path d="M39.08,16.58h-4.46c-0.75,0 -1.25,-0.54 -1.29,-1.29V10.79" class="st1"/><path d="M41.25,8.83L34.58,15.54" class="st1"/></g><g><path d="M8.29,14.13V9.67c0,-0.75,0.54,-1.25,1.29,-1.29h4.5" class="st1"/><path d="M16.21,16.25L9.5,9.54" class="st1"/></g><g><path d="M10.38,33.71h4.46c0.75,0,1.25,0.54,1.29,1.29v4.5" class="st1"/><path d="M8.29,41.58L15,34.92" class="st1"/></g><g><path d="M41.67,35.83v4.46c0,0.75,-0.54,-1.25,-1.29,-1.29h-4.5" class="st1"/><path d="M33.79,33.75L40.46,40.42" class="st1"/></g><g><line x1="41.71" y1="20.83" x2="8.38" y2="20.83" class="st1"/><line x1="41.71" y1="25" x2="8.38" y2="25" class="st1"/><line x1="41.71" y1="29.17" x2="8.38" y2="29.17" class="st1"/></g></g></svg> |
        | **leaf** / **switch**       | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st1{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}</style><rect width="50" height="50" class="st0"/><g><g><path d="M38.13,11.38l3.17,3.17c0.54,0.54,0.54,1.29,0,1.79l-3.17,3.21" class="st1"/><path d="M11.88,19.54l-3.17,-3.17c-0.54,-0.54,-0.54,-1.29,0,-1.79l3.17,-3.21" class="st1"/></g><g><path d="M38.13,30.46l3.17,3.17c0.54,0.54,0.54,1.29,0,1.79l-3.17,3.21" class="st1"/><path d="M11.88,38.63l-3.17,-3.17c-0.54,-0.54,-0.54,-1.29,0,-1.79l3.17,-3.21" class="st1"/></g><g><path d="M40.25,15.33H28.29l-6.67,19.13H9.67" class="st1"/><path d="M40.25,34.46H28.29l-6.67,-19.13H9.67" class="st1"/></g></g></svg> |
        | **spine**      | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st1{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}</style><rect width="50" height="50" class="st0"/><g><g><path d="M40.83,12.54H28.33L21.67,37.46H9.17" class="st1"/><path d="M11.67,41.67l-2.92,-3.38c-0.54,-0.54 -0.54,-1.29 0,-1.79l2.92,-3.17" class="st1"/><path d="M38.33,8.33l2.92,3.38c0.54,0.54 0.54,1.29 0,1.79L38.33,16.67" class="st1"/></g><g><path d="M40.83,37.46H26.67" class="st1"/><path d="M38.33,33.33l2.92,3.17c0.54,0.54 0.54,1.29 0,1.79l-2.92,3.38" class="st1"/></g><g><path d="M23.33,12.54H9.17" class="st1"/><path d="M11.67,16.67l-2.92,-3.17c-0.54,-0.54 -0.54,-1.29 0,-1.79l2.92,-3.38" class="st1"/></g><g><line x1="41.67" y1="25" x2="30" y2="25" class="st1"/><line x1="8.33" y1="25" x2="20" y2="25" class="st1"/></g></g></svg> |
        | **server**        | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st1{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}</style><rect width="50" height="50" class="st0"/><g><path d="M35.38,39.58H14.63c-0.46,0-0.83-0.38-0.83-0.83V11.25c0-0.46,0.38-0.83,0.83-0.83h20.71c0.46,0,0.83,0.38,0.83,0.83V38.75C36.21,39.21,35.83,39.58,35.38,39.58z" class="st1"/><line x1="14.63" y1="17.21" x2="32.79" y2="17.21" class="st1"/><line x1="14.63" y1="32.79" x2="32.79" y2="32.79" class="st1"/><line x1="14.63" y1="27.58" x2="32.79" y2="27.58" class="st1"/><line x1="14.63" y1="22.42" x2="32.79" y2="22.42" class="st1"/></g></svg> |
        | **pon**        | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st1{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}.st2{fill:#FFFFFF;stroke:#FFFFFF;stroke-width:1.67;stroke-miterlimit:10;}</style><rect width="50" height="50" class="st0"/><g><polyline points="8.71,8.33 40.42,25 8.71,41.67" class="st1"/><line x1="8.71" y1="25" x2="30.75" y2="25" class="st1"/><circle cx="39.63" cy="25" r="1.25" class="st2"/></g></svg> |
        | **controller**        | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st1{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}</style><rect width="50" height="50" class="st0"/><g><g><path d="M34.5,25c0,5.25-4.25,9.5-9.5,9.5S15.5,30.25,15.5,25S19.75,15.5,25,15.5c2.63,0,5,1.08,6.75,2.79C33.42,20.04,34.5,22.42,34.5,25z" class="st1"/><g><path d="M38.5,11.58l2.79,3c0.5,0.5,0.5,1.21,0,1.71l-2.79,3.21" class="st1"/><line x1="24.92" y1="15.5" x2="40.79" y2="15.5" class="st1"/></g></g><g><path d="M11.5,38.42L8.71,35.42c-0.5,-0.5,-0.5,-1.21,0,-1.71l2.79,-3.21" class="st1"/><line x1="25.08" y1="34.5" x2="9.21" y2="34.5" class="st1"/></g></g></svg> |
        | **rgw**        | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st1{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}.st2{fill:#FFFFFF;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:10;}</style><rect width="50" height="50" class="st0"/><path d="M25,31.21c0.33,0,0.63,-0.29,0.63,-0.63c0,-0.33,-0.29,-0.63,-0.63,-0.63c-0.33,0,-0.63,0.29,-0.63,0.63C24.38,30.92,24.67,31.21,25,31.21z" class="st2"/><path d="M19.5,22.75c3.13,-3.13,8.33,-3.13,11.46,0" class="st1"/><path d="M22.38,26.63c1.58,-1.58,4.17,-1.58,5.75,0" class="st1"/><g><path d="M7.08,23.17l15.71,-15.21c1.29,-1.25,3.38,-1.25,4.67,0L42.92,23.21" class="st1"/><path d="M12.46,26.58v13c0,1.83,1.5,3.33,3.33,3.33h7.46c1,0,1.79,-0.79,1.79,-1.79v-3.54" class="st1"/><path d="M37.63,26.58v13c0,1.83,-1.5,3.33,-3.33,3.33h-3.54" class="st1"/></g></svg> |
        | **client**        | <svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" viewBox="0 0 50 50" fill="none"><style type="text/css">.st0{fill:#001135;}.st4{fill:none;stroke:#FFFFFF;stroke-width:1.67;stroke-linecap:round;stroke-linejoin:round;}</style><rect width="50" height="50" class="st0"/><path class="st4" d="M41.67,37.96H8.33H41.67z M37.13,13.54c0,-0.21,0,-0.42,-0.08,-0.58c-0.08,-0.21,-0.17,-0.38,-0.33,-0.50c-0.13,-0.13,-0.33,-0.25,-0.50,-0.33c-0.21,-0.08,-0.38,-0.08,-0.58,-0.08H14.42c-0.21,0,-0.42,0,-0.58,0.08c-0.21,0.08,-0.38,0.17,-0.50,0.33c-0.13,0.13,-0.25,0.33,-0.33,0.50c-0.08,0.21,-0.08,0.38,-0.08,0.58V31.67h24.25V13.54z"/></svg> | 

!!!warning
      when label topoViewer-role value is not defined, it will be defaulted to **pe**

////
//// tab | group

  - **Value Format:** `string`
  - **Alias:** `topoViewer-group`
  - **Purpose:** Categorizes nodes into specific groups (e.g., "Data Center Spine", "Data Center Leaf") to enhance the structure and readability of the topology.

////
//// tab | graph-level

  - **Value Format:** Positive `integer`
  - **Alias:** `graph-leveltopoViewer-groupLevel`
  - **Purpose:** Works in conjunction with `topoViewer-group` to define the hierarchical level of nodes:
    - **Vertical Layout:** Level 1 nodes appear at the top, with higher numbers positioned lower.
    - **Horizontal Layout:** Level 1 nodes appear on the left, with higher numbers positioned to the right.

////
//// tab | geo-coordinate-lat / geo-coordinate-lng

  - **Value Format:** `string`
  - **Alias:** `topoViewer-geoCoordinateLat` and `topoViewer-geoCoordinateLng`
  - **Purpose:** Define the geographic latitude and longitude for node positioning in a geo-based layout. If omitted, TopoViewer assigns random positions.

////
///


#### Example Containerlab Topology Definition

Below is an example Containerlab topology definition that utilizes these labels to enhance the visualization

//// tab | Visualization output
Below is an example screenshot illustrating how TopoViewer displays the labeled topology
![topoviewer-labeled-topology](https://github.com/user-attachments/assets/f8c75b7f-36aa-46d3-865b-3f6a25ac52dc)
////
//// tab | Topology definition
/// details | 
```yaml
name: nokia-DataCenter-lab

topology:
  nodes:
  
    Spine-01:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: spine
        topoViewer-group: "Data Center Spine"
        topoViewer-groupLevel: 2

    Spine-02:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: spine
        topoViewer-group: "Data Center Spine"
        topoViewer-groupLevel: 2
        topoViewer-geoCoordinateLat: "52.532161628640615"
        topoViewer-geoCoordinateLng: "13.420430194846846"

    Leaf-01:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: server
        topoViewer-group: "Data Center Leaf"
        topoViewer-groupLevel: 3
        topoViewer-geoCoordinateLat: "51.45664108633426"
        topoViewer-geoCoordinateLng: "7.00441511803141"

    Leaf-02:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: client
        topoViewer-group: "Data Center Leaf"
        topoViewer-groupLevel: 3
        topoViewer-geoCoordinateLat: "51.53871503745607"
        topoViewer-geoCoordinateLng: "7.564717804534128"

    Leaf-03:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: controller
        topoViewer-group: "Data Center Leaf"
        topoViewer-groupLevel: 3
        topoViewer-geoCoordinateLat: "51.326388273344435"
        topoViewer-geoCoordinateLng: "9.49831138932782"

    Leaf-04:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: rgw
        topoViewer-group: "Data Center Leaf"
        topoViewer-groupLevel: 3
        topoViewer-geoCoordinateLat: "51.09927769956055"
        topoViewer-geoCoordinateLng: "13.980732881349564"

    BorderLeaf-01:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: switch
        topoViewer-group: "Data Center Border Leaf"
        topoViewer-groupLevel: 2
        topoViewer-geoCoordinateLat: "54.318988964885484"
        topoViewer-geoCoordinateLng: "10.190450002066472"

    BorderLeaf-02:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: switch
        topoViewer-group: "Data Center Border Leaf"
        topoViewer-groupLevel: 2
        topoViewer-geoCoordinateLat: "54.168316500414994"
        topoViewer-geoCoordinateLng: "12.311934204350786"

    DCGW-01:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: pe
        topoViewer-group: "Data Center DCGW-01"
        topoViewer-groupLevel: 1
        topoViewer-geoCoordinateLat: "54.318988964885484"
        topoViewer-geoCoordinateLng: "10.190450002066472"

    DCGW-02:
      kind: srl
      image: ghcr.io/nokia/srlinux:24.10.2-357-arm64
      labels:
        topoViewer-role: router
        topoViewer-group: "Data Center DCGW-02"
        topoViewer-groupLevel: 1
        topoViewer-geoCoordinateLat: "54.168316500414994"
        topoViewer-geoCoordinateLng: "12.311934204350786"

  links:

    - endpoints: ["Spine-01:e1-1", "Leaf-01:e1-1"]
    - endpoints: ["Spine-01:e1-2", "Leaf-02:e1-1"]
    - endpoints: ["Spine-01:e1-3", "Leaf-03:e1-3"]
    - endpoints: ["Spine-01:e1-4", "Leaf-04:e1-3"]
    - endpoints: ["Spine-01:e1-5", "BorderLeaf-01:e1-1"]
    - endpoints: ["DCGW-01:e1-1", "BorderLeaf-01:e1-2"]


    - endpoints: ["Spine-02:e1-1", "Leaf-01:e1-2"]
    - endpoints: ["Spine-02:e1-2", "Leaf-02:e1-2"]
    - endpoints: ["Spine-02:e1-3", "Leaf-03:e1-4"]
    - endpoints: ["Spine-02:e1-4", "Leaf-04:e1-4"]
    - endpoints: ["Spine-02:e1-5", "BorderLeaf-02:e1-1"]
    - endpoints: ["DCGW-02:e1-1", "BorderLeaf-02:e1-2"]
```
///
////



With these enhancements, TopoViewer transforms your Containerlab topology into a clear, intuitive, and manageable network topology.


### Packet capture

In the explorer you can expand running labs and containers to view all the discovered interfaces for that container. You can either right click on the interface and start a packet capture or click the shark icon on the interface label.

Packet capture relies on the [Edgeshark integration](wireshark.md#edgeshark-integration). Install Edgeshark first, and then use the context menu or the fin icon next to the interface name.

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

The time interval (in milliseconds) for which the extension automatically refreshes.

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
