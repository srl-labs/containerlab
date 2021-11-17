Node object is one of the containerlab' pillars. Essentially, it is nodes and links what constitute the lab topology. To let users build flexible and customizable labs the nodes are meant to be configurable.

The node configuration is part of the [topology definition file](topo-def-file.md) and **may** consist of the following fields that we explain in details below.

```yaml
# part of topology definition file
topology:
  nodes:
    node1:  # node name
      kind: srl
      type: ixrd2
      image: ghcr.io/nokia/srlinux
      startup-config: /root/mylab/node1.cfg
      binds:
        - /usr/local/bin/gobgp:/root/gobgp
        - /root/files:/root/files:ro
      ports:
      - 80:8080
      - 55555:43555/udp
      - 55554:43554/tcp
      user: test
      env:
        ENV1: VAL1
      cmd: /bin/bash script.sh
```

### kind
The `kind` property selects which kind this node is of. Kinds are essentially a way of telling containerlab how to treat the nodes properties considering the specific flavor of the node. We dedicated a [separate section](kinds/kinds.md) to discuss kinds in details.

!!!note
    Kind **must** be defined either by setting the kind for a node specifically (as in the example above), or by setting the default kind:
    ```yaml
    topology:
      defaults:
        kind: srl
      nodes:
        node1:
        # kind value of `srl` is inherited from defaults section
    ```

### type
With `type` the user sets a type of the node. Types work in combination with the kinds, such as the type value of `ixrd2` sets the chassis type for SR Linux node, thus this value only makes sense to nodes of kind `srl`.

Other nodes might treat `type` field differently, that will depend on the kind of the node. The `type` values and effects defined in the documentation for a specific kind.

### image
The common `image` attribute sets the container image name that will be used to start the node. The image name should be provided in a well-known format of `repository(:tag)`.

We use `<repository>` image name throughout the docs articles. This means that the image with `<repository>:latest` name will be looked up. A user will need to add the latest tag if they want to use the same loose-tag naming:

```bash
# tagging srlinux:20.6.1-286 as srlinux:latest
# after this change its possible to use `srlinux:latest` or `srlinux` image name
docker tag srlinux:20.6.1-286 srlinux:latest
```

### license
Some containerized NOSes require a license to operate or can leverage a license to lift-off limitations of an unlicensed version. With `license` property a user sets a path to a license file that a node will use. The license file will then be mounted to the container by the path that is defined by the `kind/type` of the node.

### startup-config
For some kinds it's possible to pass a path to a config file that a node will use on start instead of a bare config. Check documentation for a specific kind to see if `startup-config` element is supported.

Note, that if a config file exists in the lab directory for a given node, then it will take preference over the startup config passed with this setting. If it is desired to discard the previously saved config and use the startup config instead, use the `enforce-startup-config` setting or deploy a lab with the [`reconfigure`](../cmd/deploy.md#reconfigure) flag.

### enforce-startup-config
By default, containerlab will use the config file that is available in the lab directory for a given node even if the `startup config` parameter points to another file. To make a node to boot with the config set with `startup-config` parameter no matter what, set the `enforce-startup-config` to `true`.

### startup-delay
To make certain node(s) to boot/start later than others use the `startup-delay` config element that accepts the delay amount in seconds.

This setting can be applied on node/kind/default levels.

### binds
In order to expose host files to the containerized nodes a user can leverage the bind mount capability.

Provide a list of binds instructions under the `binds` container of the node configuration. The string format of those binding instructions follow the same rules as the [--volume parameter](https://docs.docker.com/storage/volumes/#choose-the--v-or---mount-flag) of the docker/podman CLI.

```yaml
binds:
  # mount a file from a host to a container (implicit RW mode)
  - /usr/local/bin/gobgp:/root/gobgp
  # mount a directory from a host to a container in RO mode
  - /root/files:/root/files:ro
```

???info "Bind variables"
    By default binds are either provided as an absolute, or a relative (to the current working dir) path. Although the majority of cases can be very well covered with this, there are situations in which it is desirable to use a path that is relative to the node-specific example.

    Consider a two-node lab `mylab.clab.yml` that has node-specific files, such as state information or some additional configuration artifacts. A user could create a directory for such files similar to that:

    ```
    .
    ├── cfgs
    │   ├── n1
    │   │   └── conf
    │   └── n2
    │       └── conf
    └── mylab.clab.yml

    3 directories, 3 files
    ```

    Then to mount those files to the nodes, the nodes would have been configured with binds like that:

    ```yaml
    name: mylab
    topology:
      nodes:
        n1:
          binds:
            - cfgs/n1/conf:/conf
        n2:
          binds:
            - cfgs/n2/conf:/conf
    ```

    while this configuration is perfectly fine, it might be considered verbose as the number of nodes grow. To remove this verbosity, the users can leverage a special variable `$nodeDir` in their bind paths. This variable will expand to the node specific directory that containerlab creates for each node.

    What this means is that you can create a directory structure that containerlab will create anyhow and put the needed files over there. With the lab named `mylab` and the nodes named `n1` and `n2` the structure containerlab uses is as follows:

    ```
    .
    ├── clab-mylab
    │   ├── n1
    │   │   └── conf
    │   └── n2
    │       └── conf
    └── mylab.clab.yml

    3 directories, 3 files
    ```

    With this structure in place, the clab file can leverage the `$nodeDir` variable:

    ```yaml
    name: mylab
    topology:
      nodes:
        n1:
          binds:
            - $nodeDir/conf:/conf
        n2:
          binds:
            - $nodeDir/conf:/conf
    ```

    Notice how `$nodeDir` hides the directory structure and node names and removes the verbosity of the previous approach.

### ports
To bind the ports between the lab host and the containers the users can populate the `ports` object inside the node:

```yaml
ports:
  - 80:8080 # tcp port 80 of the host is mapped to port 8080 of the container
  - 55555:43555/udp
  - 55554:43554/tcp
```
The list of port bindings consists of strings in the same format that is acceptable by `docker run` command's [`-p/--export` flag](https://docs.docker.com/engine/reference/commandline/run/#publish-or-expose-port--p---expose).

This option is only configurable under the node level.

### env
To add environment variables to a node use the `env` container that can be added at `defaults`, `kind` and `node` levels.

The variables values are merged when the same vars are defined on multiple levels with nodes level being the most specific.

```yaml
topology:
  defaults:
    env:
      ENV1: 3 # ENV1=3 will be set if its not set on kind or node level
      ENV2: glob # ENV2=glob will be set for all nodes
  kinds:
    srl:
      env:
        ENV1: 2 # ENV1=2 will be set to if its not set on node level
        ENV3: kind # ENV3=kind will be set for all nodes of srl kind
  nodes:
    node1:
      env:
        ENV1: 1 # ENV1=1 will be set for node1
        # env vars expansion is available, for example
        # ENV2 variable will be set to the value of the environment variable SOME_ENV
        # that is defined for the shell you run containerlab with
        ENV2: ${SOME_ENV} 
```

You can also specify a magic ENV VAR - `__IMPORT_ENVS: true` - which will import all environment variables defined in your shell to the relevant topology level.

### user
To set a user which will be used to run a containerized process use the `user` configuration option. Can be defined at `node`, `kind` and `global` levels.

```yaml
topology:
  defaults:
    user: alice # alice user will be used for all nodes unless set on kind or node levels
  kinds:
    srl:
      user: bob # bob user will be used for nodes of kind srl unless it is set on node level
  nodes:
    node1:
      user: clab # clab user will be used for node1
```

### entrypoint
Changing the entrypoint of the container is done with `entrypoint` config option. It accepts the "shell" form and can be set on all levels.

```yaml
topology:
  defaults:
    entrypoint: entrypoint.sh
  kinds:
    srl:
      cmd: entrypoint.sh
  nodes:
    node1:
      cmd: entrypoint.sh
```

### cmd
It is possible to set/override the command of the container image with `cmd` configuration option. It accepts the "shell" form and can be set on all levels.

```yaml
topology:
  defaults:
    cmd: bash cmd.sh
  kinds:
    srl:
      cmd: bash cmd2.sh
  nodes:
    node1:
      cmd: bash cmd3.sh
```

### labels
To add container labels to a node use the `labels` container that can be added at `defaults`, `kind` and `node` levels.

The label values are merged when the same vars are defined on multiple levels with nodes level being the most specific.

Consider the following example, where labels are defined on different levels to show value propagation.
```yaml
topology:
  defaults:
    labels:
      label1: value1
      label2: value2
  kinds:
    srl:
      labels:
        label1: kind_value1
        label3: value3
  nodes:
    node1:
      labels:
        label1: node_value1
```

As a result of such label distribution, node1 will have the following labels:

```bash
label1: node_value1 # most specific label wins
label2: value2 # inherited from defaults section
label3: value3 # inherited from kinds section
```

### mgmt_ipv4
To make a node to boot with a user-specified management IPv4 address, the `mgmt_ipv4` setting can be used. Note, that the static management IP address should be part of the subnet that is used within the lab.

Read more about user-defined management addresses [here](network.md#user-defined-addresses).

```yaml
nodes:
    r1:
      kind: srl
      mgmt_ipv4: 172.20.20.100
```

### mgmt_ipv6
To make a node to boot with a user-specified management IPv4 address, the `mgmt_ipv6` setting can be used. Note, that the static management IP address should be part of the subnet that is used within the lab.

Read more about user-defined management addresses [here](network.md#user-defined-addresses).

```yaml
nodes:
    r1:
      kind: srl
      mgmt_ipv6: 2001:172:20:20::100
```

### publish
Container lab integrates with [mysocket.io](https://mysocket.io) service to allow for private, Internet-reachable tunnels created for ports of containerlab nodes. This enables effortless access sharing with customers/partners/colleagues.

This integration is extensively covered on [Publish ports](published-ports.md) page.

```yaml
name: demo
topology:
  nodes:
    r1:
      kind: srl
      publish:
        - tcp/22     # tcp port 22 will be published
        - tcp/57400  # tcp port 57400 will be published
        - http/8080  # http port 8080 will be published
```

### network-mode
By default containerlab nodes use bridge-mode driver - nodes are created with their first interface connected to a docker network (management network).

It is possible to override this behavior and set the network mode to the value of `host`.

```yaml
# example node definition with host networking mode
my-node:
  image: alpine:3
  network-mode: host
```

The `network-mode` configuration option set to `host` will launch the node in the [host networking mode](https://docs.docker.com/network/host/).

### runtime
By default containerlab nodes will be started by `docker` container runtime. Besides that, containerlab has experimental support for `containerd` and `ignite` runtimes.

It is possible to specify a global runtime with a global `--runtime` flag, or set the runtime on a per-node basis:

Options for the runtime parameter are:
- `docker`
- `containerd`
- `ignite`

The default runtime can also be influenced via the `CLAB_RUNTIME` environment variable, which takes the same values as mentioned above.

```yaml
# example node definition with per-node runtime definition
my-node:
  image: alpine:3
  runtime: containerd
```

### exec
Containers typically have some process that is launched inside the sandboxed environment. The said process and its arguments are provided via container instructions such as `entrypoint` and `cmd` in Docker's case.

Quite often, it is needed to run additional commands inside the containers when they finished booting. Instead of modifying the `entrypoint` and `cmd` it is possible to use the `exec` parameter and specify a list of commands to execute:

```yaml
# two commands will be executed for node `my-node` once it finishes booting
my-node:
  image: alpine:3
  kind: linux
  binds:
    - myscript.sh:/myscript.sh
  exec:
    - echo test123
    - bash /myscript.sh
```

The `exec` is particularly helpful to provide some startup configuration for linux nodes such as IP addressing and routing instructions.

### memory

By default, container runtimes do not impose any memory resource constraints[^1].
A container can use too much of the host's memory, making the host OS unstable.

The `memory` parameter can be used to limit the amount of memory a node/container can use.

```yaml
# my-node will have access to at most 1Gb of memory.
my-node:
  image: alpine:3
  kind: linux
  memory: 1Gb
```

Supported memory suffixes (case insensitive): `b`, `kib`, `kb`, `mib`, `mb`, `gib`, `gb`.

### cpu

By default, container runtimes do not impose any CPU resource constraints[^1].
A container can use as much as the host's scheduler allows.

The `cpu` parameter can be used to limit the number of CPUs a node/container can use.

```yaml
# my-node will have access to at most 1.5 of the CPUs
# available in the host machine.
my-node:
  image: alpine:3
  kind: linux
  cpu: 1.5
```

### cpu-set

The `cpu-set` parameter can be used to limit the node CPU usage to specific cores of the host system.

Valid syntaxes:

- `0-3`: Cores 0, 1, 2 and 3
- `0,3`: Cores 0 and 3
- `0-1,4-5`: Cores 0, 1, 4 and 5

```yaml
# my-node will have access to CPU cores 0, 1, 4 and 5.
my-node:
  image: alpine:3
  kind: linux
  cpu-set: 0-1,4-5
```

[^1]: [docker runtime resources constraints](https://docs.docker.com/config/containers/resource_constraints/).