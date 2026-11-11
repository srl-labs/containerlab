# tui command

## Description

The `tui` command opens a full-screen terminal UI to watch and operate labs without leaving the terminal.

It shows live tables of labs, nodes, interfaces and links, and runs the usual operations from the keyboard:

* deploy, destroy and redeploy labs, with the output of the command streamed into the UI;
* start, stop and restart nodes, save their configuration, run a command in many nodes at once;
* open a shell or an SSH session on a node — the UI steps aside while the session runs and comes back when it ends;
* follow and search the logs of a node;
* watch the traffic rate of every interface and link, and the CPU and memory use of every node;
* set and clear link impairments (delay, jitter, loss, rate, corruption);
* capture packets and watch them live in the UI, in Wireshark or keep them in a file;
* share a terminal with [sshx](tools/sshx/attach.md) or [GoTTY](tools/gotty/attach.md).

Running `containerlab` without a command opens the same UI when it is run in a terminal. Scripts are not affected: outside of a terminal the help is printed, as before. Set the `CLAB_NO_TUI` environment variable, ie. `CLAB_NO_TUI=1`, to get the help in a terminal as well; `CLAB_NO_TUI=0` or `false` leaves the UI on.

## Usage

`containerlab [global-flags] tui [local-flags]`

## Privileges

The UI itself needs no more than access to the container runtime. Everything that changes a lab is done by running the corresponding `containerlab` command in the background, which obtains its privileges the way it does when you run it yourself.

When root privileges cannot be obtained — containerlab is neither installed with the SUID bit nor run with `sudo` — the UI still starts and can be used to watch labs, read logs, run commands in nodes and open sessions. The operations that need root are then unavailable and say so, and the status line shows `read-only (no root)`.

## Labs in a Kubernetes cluster

With the [c9s runtime](../manual/clabernetes/runtime.md) the UI shows the labs that run in a Kubernetes cluster instead of the ones of this host:

```bash
containerlab -r c9s tui
```

It lists the labs of every namespace you may read, or of one namespace with the global `--namespace` flag, and uses the cluster your kubeconfig selects, like the other commands do. Nothing needs root privileges: what you can do is what the cluster lets you do.

The views and the keys are the same. A few things differ, because the nodes are pods and not containers of this host:

* The state and the traffic of the interfaces are read from inside the nodes every two seconds. CPU and memory use come from the metrics of the cluster and are missing when it serves none (no metrics-server).
* A shell on a node is opened through the cluster. SSH connects through a port forwarded to the node, as the user the kind of the node logs in with by default.
* A lab that was deployed from another computer has no topology file here. Its links are the ones the cluster reports for the running nodes, and it can be watched, stopped, saved, captured on and destroyed, but not redeployed.
* Destroying a lab keeps the saved configuration of its nodes in the cluster, unless you tick the option to remove it.
* Link impairments and terminal sharing work on the containers and the network of this host. They are not available, and say so.
* Two labs of the same name in different namespaces are listed as `name@namespace`.

## Navigating

The first line holds the views, the second one tells where you are, and the last two list the keys of the current context and show notifications.

Five keys are enough to get anywhere:

| Key | Action |
| --- | --- |
| `↑` `↓` | move to a lab, a node, an interface |
| `enter` | open it: a lab opens its nodes, a node its interfaces |
| `esc` | go back |
| `a` | list everything that can be done with the selection, and do it |
| `/` | find something by typing a part of its name |

The UI recalls them on start, the last but one line always shows the keys of the current selection, and `?` lists all of them. Everything else is a shortcut for one of these steps:

| Key | Action |
| --- | --- |
| `→` `←` | the same as `enter` and `esc`, so that the arrows alone walk through labs, nodes and interfaces |
| `[` `]` | previous and next lab, or node when one is open, without going back to the list |
| `ctrl+k` | go to a lab, a node or a view by typing a part of its name |
| `1` … `6` | switch to the labs, nodes, interfaces, links, map or jobs view |
| `0` | show all labs again after having opened one |
| `:` | command line: a view, the name of a lab or a node, `:theme <name>`, `:messages`, `:quit` |
| `q` | go back, and quit from the list of labs |
| `/` | filter the list, or find text in logs; several words must all match and `!word` excludes |
| `<` `>` `I` | sort by the previous or next column, invert the order |
| `space` | mark a row; actions then apply to all marked rows |
| `ctrl+a` | mark or unmark all rows |
| `p` | show or hide the details pane |
| `?` | list all keys |
| `ctrl+z` | suspend the UI and return to the shell, `fg` brings it back |
| `ctrl+l` | redraw the screen |
| `ctrl+c` | quit |

`esc` undoes one thing at a time: it clears the filter, then the marks, and then leaves the view. Each view keeps its cursor, filter and sort order while you visit other views. The list of labs always shows all labs; the other views show the lab that was opened, or all of them after `0`.

### Menus and dialogs

Menus are answered with a single key: every entry has its key written in front of it, the arrows and `enter` work as well, and `/` searches a long menu. Dialogs are confirmed with `y` and cancelled with `n` or `esc`; `tab` moves between their fields and `space` toggles an option.

A typical session needs few keys: `enter` on a lab and `s` on a node open a shell, `X` and `y` destroy the selected lab, `y` and `4` copy the IPv4 address of a node.

### Mouse

The mouse is an addition, nothing depends on it:

* click a row to select it, double-click to open it, right-click for the menu of its actions;
* click a view in the first line, a column title to sort by it, or a key hint in the last line but one to do what the key does;
* scroll lists, logs and the map with the wheel, drag the map to move it;
* click fields and buttons in dialogs, click next to a menu to close it.

Hold `shift` while dragging to select text with the terminal instead, or start the UI with `--no-mouse`.

## Actions

Keys that change something are upper case and ask for confirmation, showing the command that is about to run. The last lines of the screen list the keys that apply to the current selection, and `a` opens the same list as a menu that also tells why an action is unavailable.

| On | Key | Action |
| --- | --- | --- |
| lab | `D` | deploy; on a deployed lab this applies the changes of the topology file |
| lab | `X` | destroy |
| lab | `R` | redeploy |
| lab | `W` | save the configuration of all nodes |
| lab | `x` | run a command in all nodes |
| lab | `H` | share a terminal, copy the sharing link |
| lab, node | `t` | [topology map](#topology-map) |
| node | `s` / `S` | shell with the default of the node's kind / SSH |
| node | `c` | choose how to connect |
| node | `l` | logs |
| node | `d` | describe |
| node | `x` | run a command |
| node | `U` / `O` / `R` | start / stop / restart |
| node | `W` | save the configuration |
| node | `y` | copy a value: name, addresses, image, … |
| interface, link | `n` / `N` | impair: set delay, jitter, loss, rate and corruption / remove the impairments |
| interface, link | `C` | [capture packets](#packet-capture) |
| interface | `m` | copy the MAC address |
| job | `C` | cancel |

Copying uses the clipboard of your terminal, so it also works when the UI runs on a remote host over SSH, provided the terminal supports it.

## Packet capture

`C` on an interface or a link starts a capture. The dialog asks where the packets go:

| Send to | What happens |
| --- | --- |
| `wireshark` | Wireshark opens on the host and shows the packets live. Offered when Wireshark is installed and a graphical display is available. |
| `live view` | The packets are decoded in the UI, one line each, as they cross the link. |
| `live view + file` | The same, and the capture is written to a pcapng file. |
| `file` | The capture is written to a pcapng file, to open later. |
| `my computer` | Nothing is captured yet: the UI gives you a command to run on your own computer, which runs the capture through SSH and pipes it into the Wireshark there. Offered when the UI is used through SSH. Unless containerlab is installed with the setuid bit and you are a member of the `clab_admins` group, the command runs containerlab with `sudo -n`, which needs sudo without a password on the host: SSH gives sudo no terminal to ask for one. |

The live view lists the time, the interface, the addresses, the protocol and a summary of every packet. It follows the newest packet until you move up; `f`, the space bar or `G` follow again. `⏎` shows the protocol headers and the bytes of the packet under the cursor, and `/` filters the list, ie. by `bgp`, an address or `!lldp`. `C` stops the capture, and the packets stay to look at; `l` shows what the capture command logged. The capture is a job like any other: it keeps running when you leave the view, and the jobs view (`6`) leads back to it. The last 10000 packets are kept.

The live view names the common protocols of a network lab, among them ARP, LLDP, LACP, STP, ICMP, BGP, OSPF, IS-IS, BFD, VRRP, DNS and DHCP, together with the VLAN, MPLS and VXLAN encapsulations around them. It does not replace Wireshark: use one of the other choices when you need every field of a protocol.

The command for your own computer is copied to the clipboard and shown on the screen. It uses the address your SSH session came in on, which is not the one you typed when there is a jump host or address translation on the way; replace it with the name you normally use in that case.

## Topology map

The map view (`5`, or `t` on a lab or a node) draws the topology of a lab: nodes as boxes colored by their state, links as lines colored by theirs, with the traffic rate written next to them. Links that are down are drawn dashed, so that the state does not depend on color alone.

| Key | Action |
| --- | --- |
| `←` `↑` `↓` `→` | move the focus from node to node |
| `tab` | step through the links of the node in focus, `o` jumps to the far end |
| `enter` | open the node |
| `/` | go to a node by name |
| `e` | how much of the lab is drawn, see below |
| `+` `-` `f` | zoom in, zoom out, fit |
| `v` | layout: saved positions, tiers or by connectivity |
| `i` | interface names at the ends of the links: for the node in focus, for every link, off |
| `r` | show or hide the traffic rates on the links |
| `H` `J` `K` `L` | pan, also `shift` with the arrows |
| `p` | show or hide the details panel |
| `esc` | go back |

A change of the zoom, the layout or the extent is confirmed in the status line, as it does not always show in a small lab.

### Link labels

Two things are written on the links, and each can be switched off:

* the **traffic rate** in the middle of a link, toggled with `r`;
* the **names of the interfaces** at the ends of a link, each next to the node it belongs to. `i` steps through naming the links of the node in focus, which is the default, naming every link, and no names at all.

A name is written only where it cannot be mistaken: next to the line where it leaves its node, or on the line itself on the side of its node. Where the gap between two nodes is a single row, a small arrow tells the two names apart: `e1-1▴` belongs to the node above, `eth1▾` to the one below. When there is no such place, as with the many links of a spine that share one line, the name is left out; the panel next to the map lists the interfaces of the node in focus in any case.

As long as the drawing still fits the window, links are drawn with the room their texts need: two nodes next to each other move apart far enough for both names and the rate, and nodes below each other get a second line between them. A lab that only just fits is drawn tight instead, and names are then written where they find a place.

The choice is kept when the map moves on to another lab.

With a node in focus the node actions apply (`s` shell, `l` logs, …), with a link in focus the link actions do (`n` impairments, `C` capture).

Nodes are arranged in tiers. The tier of a node is taken from its `graph-level` label when there is one, else worked out from the names (`spine`, `leaf`, `core`, `pe`, …) and from how the nodes are connected. When the topology was arranged in the containerlab app or the VS Code extension, the saved positions are used to order the nodes.

### Large labs

A drawing of a large lab is many times wider than a terminal. The map therefore draws as much as fits and says in its title what it shows:

* **all** – every node, for labs that fit.
* **no hosts** – nodes connected to a single other node, typically hosts and clients, are folded into that node, which is then labeled with their number: `leaf1 +4`.
* **2 hops** – the node in focus and what is at most two links away from it.
* **neighbors** – the node in focus and the nodes it connects to. Moving the focus redraws the map around the new node, so a lab of any size can be walked through, and `/` jumps anywhere. A node with more neighbors than fit, like a spine, shows the ones next to where you came from.

`e` switches between these by hand; the whole lab can always be drawn and panned through. The drawing is centered on the node in focus whenever it changes.

## Flags

### theme

The UI keeps the background and the text color of the terminal and only adds colors that read well on top of them. `--theme` tells whether those are picked for a dark or a light background:

* `auto`, the default, asks the terminal.
* `dark` and `light` force one of the two, for a terminal that does not answer or answers wrong, which shows as text that is hard to read.

The theme can also be changed from within the UI with `:theme <name>`.

### labs-dir

Besides the running labs the UI lists topology files that are not deployed. It looks for them in the current directory, next to the topology files of the running labs and in `~/.clab`. With `--labs-dir` more directories are searched; the flag can be given multiple times.

### read-only

`--read-only` disables every operation that changes a lab.

### no-mouse

`--no-mouse` leaves the mouse to the terminal.

### demo

`--demo` shows simulated labs with made-up traffic instead of the real ones: a data center fabric, a service provider core, a metro ring and a campus network. It needs neither a container runtime nor root, which makes it handy for a first look.

### name

With the global `--name` flag the UI opens on the nodes of the given lab.

### Environment variables

Like for other commands, the flags can be set with environment variables: `CLAB_TUI_THEME`, `CLAB_TUI_LABS_DIR`, `CLAB_TUI_READ_ONLY`, `CLAB_TUI_NO_MOUSE`.

## Examples

```bash
# open the UI
containerlab tui

# the same, a bare command in a terminal opens the UI
containerlab

# open the UI on the nodes of one lab, with the colors for a light terminal
containerlab tui --name srl02 --theme light

# have a look without any lab
containerlab tui --demo

# watch and operate the labs of a Kubernetes cluster
containerlab -r c9s tui
```
