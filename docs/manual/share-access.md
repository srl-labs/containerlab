# Share Lab Access

Lightweight, fast to deploy, and reproducible containerlabs are meant to be shared via Git. But sometimes, you can't quite share the entire lab... Maybe the recipient doesn't have access to the container images or has other limitations that prevent them from running a copy of you lab.

Yet, you might find yourself in need to share access of your lab with one or many users. Quite often you want someone else to have a look at your lab when you found something or got stuck. Or maybe you are a lecturer and want to broadcast your lab interaction to your students.

Here we will discuss different ways how you can share your lab with others in a secure and interactive way.

1. Share the web terminal to your lab via a public SSHX server using [sshx.io](https://sshx.io) service
2. If you don't trust the SSHX server and the e2e encryption methods it uses, you can run the web terminal to your lab locally next to your lab with [GoTTY](#gotty).

## SSHX tools

[SSHX](https://sshx.io) is a web-based terminal emulator that allows you to share access to a terminal sessions with others and have a collaborative terminal experience.

Containerlab users can leverage this handy free service[^1] to share lab access with read/write and read-only access by leveraging the [**`tools sshx`**](../cmd/tools/sshx/attach.md) command set.

### Sharing the lab

You can share any given lab that you have running by providing its name or a path to its topology file. For instance, consider the following labs running on a system:

```
❯ clab ins -a
╭─────────────────────────────────────┬──────────┬──────┬──────────────────────────────┬─────────┬───────────────────╮
│               Topology              │ Lab Name │ Name │          Kind/Image          │  State  │   IPv4/6 Address  │
├─────────────────────────────────────┼──────────┼──────┼──────────────────────────────┼─────────┼───────────────────┤
│ /tmp/.clab/topo-2502707021.clab.yml │ srl      │ srl  │ nokia_srlinux                │ running │ 172.20.20.2       │
│                                     │          │      │ ghcr.io/nokia/srlinux:latest │         │ 3fff:172:20:20::2 │
╰─────────────────────────────────────┴──────────┴──────┴──────────────────────────────┴─────────┴───────────────────╯
```

To share access to this lab with others, you can use the `tools sshx attach` command and provide the lab name as an input. The lab name is simply `srl`, so here we go:

```
clab tools sshx attach -l srl
```

<div class="embed-result">
```{.log .no-copy}
16:48:47 INFO Parsing & checking topology file=topo-2502707021.clab.yml
16:48:47 INFO Pulling image ghcr.io/srl-labs/network-multitool...
16:48:47 INFO Pulling ghcr.io/srl-labs/network-multitool:latest Docker image
16:48:56 INFO Done pulling ghcr.io/srl-labs/network-multitool:latest
16:48:56 INFO Creating SSHX container clab-srl-sshx on network 'clab'
16:48:56 INFO Creating container name=clab-srl-sshx
16:49:00 INFO SSHX container clab-srl-sshx started. Waiting for SSHX link...
16:49:05 INFO SSHX successfully started link=https://sshx.io/s/7xmRrLpH2O#lek1jA1pNNRCB0
  note=
  │ Inside the shared terminal, you can connect to lab nodes using SSH:
  │ ssh admin@clab-srl-<node-name>
```
</div>

Note, the log message that goes as `SSHX successfully started` as it will also have the SSHX link in it. By pasting this link in a browser you will get a collaborative terminal session in the browser where you can open many shell windows that will belong to the `sshx` container that runs from the `ghcr.io/srl-labs/network-multitool` with lots of networking tools installed.

### Connecting to lab nodes

The great part is that this `sshx` container has access to all other lab nodes and you can refer to them by name, as they appear in your topology file. For example, we can ssh to the SR Linux container that is part of our lab:

![img](https://gitlab.com/rdodin/pics/-/wikis/uploads/38073aeb55006b57f4b5e3db1d6a230f/CleanShot_2025-03-30_at_21.46.35_2x.png)

Also, inside the sshx container you will enjoy the autocompletion of the SSH targets from your lab. Just type `ssh <tab>` and you will get the list of the nodes in your lab that you can SSH to.

### Read-only link

SSHX can generate a link that will provide **read-only** access to the shared web terminal. The read-only link will let someone to see the terminal and what happens there, but they won't be able to create terminal windows or type in commands to the opened ones.

To generate a link with a read-only component, run:

```
clab tools sshx attach --enable-readers -l srl
```

The link will have two components, separate by a comma, for example:

```
16:59:31 INFO SSHX successfully reattached link=https://sshx.io/s/MkaIiGYLq7#TfqVxBhGys4r6F,hOkWFAcC8wFqNY
```

The part before the comma will be the read-only link - `https://sshx.io/s/MkaIiGYLq7#TfqVxBhGys4r6F` and the full link gives **read/write** access.

### Detaching

When you want to disconnect the shared web terminal:

```
clab tools sshx detach -l srl
```

This will remove the sshx container.

### Listing shared links

If you want to list the shared labs you may have running:

```
clab tools sshx list
```

## Embedding SSHX in your lab

In case you want your lab to start with an SSHX container as part of it, you can add a node to your lab like this:

```yaml
name: shared-lab
topology:
  nodes:
    srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:25.3.1

    sshx:
      kind: linux
      image: ghcr.io/srl-labs/network-multitool
      exec:
        - >-
          ash -c "curl -sSf https://sshx.io/get | sh > /dev/null ;
          sshx -q --enable-readers > /tmp/sshx &
          while [ ! -s /tmp/sshx ]; do sleep 1; done && cat /tmp/sshx"
```

By adding the `sshx` node to you will get the sshx link in the lab output right in the deployment log:

```shell
# the rest of the deploy log is omitted for brevity
21:44:15 INFO Running postdeploy actions kind=nokia_srlinux node=srl
21:44:28 INFO Executed command node=sshx command="ash -c curl -sSf https://sshx.io/get | sh > /dev/null" stdout=""
21:44:28 INFO Executed command node=sshx command="ash -c sshx -q --enable-readers > /tmp/sshx &" stdout=""
21:44:28 INFO Executed command node=sshx command="cat /tmp/sshx"
  stdout=
  │ https://sshx.io/s/94eAaunO2L#CbTbaD6bqB90oU,mCIP5PtBmF41qL

21:44:28 INFO Adding host entries path=/etc/hosts
21:44:28 INFO Adding SSH config for nodes path=/etc/ssh/ssh_config.d/clab-shared-lab.conf
```

## GoTTY

[GoTTY](https://github.com/yudai/gotty) is a free and open-source tool to share your terminal as a web application. In contrast to SSHX, GoTTY does not use a relay server and runs next to your lab nodes.

This is both a blessing and a challenge - you don't have to risk your data being sent to a relay out of your control (even though sshx promises to use end-to-end encryption), but you also don't have the same level of ease of use as sshx, since you have to ensure that the receiving party can reach your host that runs containerlab and GoTTY.

Nevertheless, the more options - the better. You can explore the GoTTY commands:

* [attach](../cmd/tools/gotty/attach.md)
* [detach](../cmd/tools/gotty/detach.md)
* [list](../cmd/tools/gotty/list.md)
* [reattach](../cmd/tools/gotty/reattach.md)

[^1]: Feel free to access the security of it by googling other researches having a go at it. For a self-hosted alternative you may consider [frp](https://github.com/fatedier/frp).
