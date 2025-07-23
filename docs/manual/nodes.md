---
search:
  boost: 6
---

# Nodes

Node object is one of the containerlab' pillars. Essentially, it is nodes and links that constitute the lab topology. To let users build flexible and customizable labs the nodes are meant to be configurable.

The node configuration is part of the [topology definition file](topo-def-file.md) and **may** consist of the following fields that we explain in details below.

```yaml
# part of topology definition file
topology:
  nodes:
    node1:  # node name
      kind: nokia_srlinux
      type: ixr-d2l
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

The `kind` property selects which kind this node is of. Kinds are essentially a way of telling containerlab how to treat the nodes properties considering the specific flavor of the node. We dedicated a [separate section](kinds/index.md) to discuss kinds in details.

!!!note
    Kind **must** be defined either by setting the kind for a node specifically (as in the example above), or by setting the default kind:
    ```yaml
    topology:
      defaults:
        kind: nokia_srlinux
      nodes:
        node1:
        # kind value of `srl` is inherited from defaults section
    ```

### type

With `type` the user sets a type of the node. Types work in combination with the kinds, such as the type value of `ixr-d2l` sets the chassis type for SR Linux node, thus this value only makes sense to nodes of kind `nokia_srlinux`.

Other nodes might treat `type` field differently, that will depend on the kind of the node. The `type` values and effects defined in the documentation for a specific kind.

### group

`group` is a freeform string that denotes which group a node belongs to. This can be used to inherit values from the [groups](./topo-def-file.md#groups) container.

The grouping is also used to sort topology elements on a [graph](../cmd/graph.md#layout-and-sorting).

The inheritance model is as follows (from most specific to less specific):

```
node -> group -> kind -> defaults
```

### image

The `image` attribute sets the container image name that the container node will use. The image name should be provided in a well-known format of the `[registry]/repository[:tag]`.

For example, consider the following possible image definitions:

- `ghcr.io/nokia/srlinux:24.10` where:
    - registry: `ghcr.io`
    - repository: `nokia`
    - image: `srlinux`
    - tag: `24.10`

- `ghcr.io/nokia/srlinux`, where:
    - registry: `ghcr.io`
    - repository: `nokia`
    - image: `srlinux`
    - tag: `latest` (default tag, if not specified)

- `alpine:3`, where:
    - registry: `docker.io` (default registry, if not specified)
    - repository: `library` (default repository, if not specified)
    - image: `alpine`
    - tag: `3`

### image-pull-policy

With `image-pull-policy` a user defines the container image pull policy.

Valid values are:

- `IfNotPresent` - Pull container image if it is not already present. (If e.g. the `:latest` tag has been updated on a remote registry, containerlab will not re-pull it, if an image with the same tag is already present)
- `Never` - Do not at all try to pull the image from a registry. An error will be thrown and the execution is stopped if the image is not available locally.
- `Always` - Always try to pull the new image from a registry. An error will be thrown if pull fails. This will ensure fetching latest image version even if it exists locally.

The default value is `IfNotPresent`.

```yaml
topology:
  nodes:
    srl:
      image: ghcr.io/nokia/srlinux
      image-pull-policy: Always
```

### restart-policy

With `restart-policy` a user defines the restart policy of a container as per [docker docs](https://docs.docker.com/engine/containers/start-containers-automatically/).

Valid values are:

- `no` - Don't automatically restart the container.
- `on-failure` - Restart the container if it exits due to an error, which manifests as a non-zero exit code. The on-failure policy only prompts a restart if the container exits with a failure. It doesn't restart the container if the daemon restarts.
- `always` - Always restart the container if it stops. If it's manually stopped, it's restarted only when Docker daemon restarts or the container itself is manually restarted.
- `unless-stopped` Similar to always, except that when the container is stopped (manually or otherwise), it isn't restarted even after Docker daemon restarts.

`no` is the default restart policy value for all kinds, but `linux`. Linux kind defaults to `always`.

```yaml
topology:
  nodes:
    srl:
      image: ghcr.io/nokia/srlinux
      kind: nokia_srlinux
      restart-policy: always
    alpine:
      kind: linux
      image: alpine
      restart-policy: "no"
```

### license

Some containerized NOSes require a license to operate or can leverage a license to lift-off limitations of an unlicensed version. With `license` property a user sets a path to a license file that a node will use. The license file will then be mounted to the container by the path that is defined by the `kind/type` of the node.

### startup-config

It is possible to provide the startup configuration that the node applies on boot for most Containerlab kinds. The startup config can be provided as:

1. A path to a file that is available on the host machine and contains the config blob that the node understands.
2. An embedded config blob that is provided as a multiline string.
3. An URL (http(s) or [S3](s3-usage-example.md)) to a file that contains the config blob that the node can apply.

Read more about the usage of the startup configuration (and other ways to perform configuration management with Containerlab) in the [Configuration Management](config-mgmt.md) section.

### enforce-startup-config

By default, containerlab will use the config file that is available in the lab directory for a given node even if the `startup config` parameter points to another file. To make a node to boot with the config set with `startup-config` parameter no matter what, set the `enforce-startup-config` to `true`.

### suppress-startup-config

By default, containerlab will create a startup-config when initially creating a lab.  To prevent a startup-config file from being created (in a Zero-Touch Provisioning lab, for example), set the `suppress-startup-config` to `true`.

### auto-remove

By default, containerlab will not remove the failed or stopped nodes so that you can read their logs and understand the reason of a failure. If it is required to remove the failed/stopped nodes, use `auto-remove: true` property.

The property can be set on all topology levels.

### startup-delay

To make certain node(s) to boot/start later than others use the `startup-delay` config element that accepts the delay amount in seconds.

This setting can be applied on node/kind/default levels.

### binds

Users can leverage the bind mount capability to expose host files to the containerized nodes.

Binds instructions are provided under the `binds` container of a default/kind/node configuration section. The format of those binding instructions follows the same of the docker's [--volume parameter](https://docs.docker.com/storage/volumes/#choose-the--v-or---mount-flag).

```yaml
topology:
  nodes:
    testNode:
      kind: linux
      # some other node parameters
      binds:
        - /usr/local/bin/gobgp:/root/gobgp # (1)!
        - /root/files:/root/files:ro # (2)!
        - somefile:/somefile # (3)!
        - ~/.ssh/id_rsa:/root/.ssh/id_rsa # (4)!
        - /var/run/somedir # (5)!
```

1. mount a host file found by the path `/usr/local/bin/gobgp` to a container under `/root/gobgp` (implicit RW mode)
2. mount a `/root/files` directory from a host to a container in RO mode
3. when a host path is given in a relative format, the path is considered relative to the topology file and not a current working directory.
4. The `~` char will be expanded to a user's home directory.
5. mount an anonymous volume to a container under `/var/run/somedir` (implicit RW mode)

/// details | Bind variables
By default, binds are either provided as an absolute or a relative (to the current working dir) path. Although the majority of cases can be very well covered with this, there are situations in which it is desirable to use a path that is relative to the node-specific example.

Consider a two-node lab `mylab.clab.yml` with node-specific files, such as state information or additional configuration artifacts. A user could create a directory for such files similar to that:

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

while this configuration is correct, it might be considered verbose as the number of nodes grows. To remove this verbosity, the users can use a special variable `__clabNodeDir__` in their bind paths. This variable will expand to the node-specific directory that containerlab creates for each node.

This means that you can create a directory structure that containerlab will create anyhow and put the needed files over there. With the lab named `mylab` and the nodes named `n1` and `n2` the structure containerlab uses is as follows:

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

With this structure in place, the clab file can leverage the `__clabNodeDir__` variable:

```yaml
name: mylab
topology:
  nodes:
    n1:
      binds:
        - __clabNodeDir__/conf:/conf
    n2:
      binds:
        - __clabNodeDir__/conf:/conf
```

Notice how `__clabNodeDir__` hides the directory structure and node names and removes the verbosity of the previous approach.

Another special variable the containerlab topology file can use is `__clabDir__`. In the example above, it would expand into `clab-mylab` folder. With `__clabDir__` variable it becomes convenient to bind files like `ansible-inventory.yml` or `topology-data.json` that containerlab automatically creates:

```yaml
name: mylab
topology:
  nodes:
    ansible:
      binds:
        - __clabDir__/ansible-inventory.yml:/ansible-inventory.yml:ro
    graphite:
      binds:
        - __clabDir__/topology-data.json:/htdocs/clab/topology-data.json:ro
```

///

Binds defined on multiple levels (defaults -> kind -> node) will be merged with the duplicated values removed (the lowest level takes precedence).

When a bind with the same destination is defined on multiple levels, the lowest level takes precedence. This allows to override the binds defined on the higher levels.

### ports

To bind the ports between the lab host and the containers the users can populate the `ports` object inside the node:

```yaml
ports:
  - 80:8080 # tcp port 80 of the host is mapped to port 8080 of the container
  - 55555:43555/udp
  - 55554:43554/tcp
```

The list of port bindings consists of strings in the same format that is acceptable by `docker run` command's [`-p/--expose` flag](https://docs.docker.com/reference/cli/docker/container/run/#publish).

This option is only configurable under the node level.

### env

To add environment variables to a node use the `env` container that can be added at `defaults`, `kind` and `node` levels.

The variables values are merged when the same vars are defined on multiple levels with nodes level being the most specific.

```yaml
topology:
  defaults:
    env:
      ENV1: 3 # ENV1=3 will be set if it's not set on kind or node level
      ENV2: glob # ENV2=glob will be set for all nodes
  kinds:
    nokia_srlinux:
      env:
        ENV1: 2 # ENV1=2 will be set to if it's not set on node level
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

/// admonition | `NO_PROXY` variable
    type: subtle-note
If you use an http(s) proxy on your host, you typically set the `NO_PROXY` environment variable in your containers to ensure that when containers talk to one another, they don't send traffic through the proxy, as that would lead to broken communication. And setting those env vars is tedious.

Containerlab automates this process by automatically setting `NO_PROXY`/`no_proxy` environment variables in the containerlab nodes with the values of:

1. `localhost,127.0.0.1,::1,*.local`
2. management network range for v4 and v6 (e.g. `172.20.20.0/24`)
3. IPv4/IPv6 management addresses of the nodes of the lab
4. node names as stated in your topology file
///

### env-files

To add environment variables defined in a file use the `env-files` property that can be defined at `defaults`, `kind` and `node` levels.

The variable defined in the files are merged across all of them wtit more specific definitions overwriting less specific. Node level is the most specific one.

Files can either be specified with their absolute path or a relative path. The base path for the relative path resolution is the directory that holds the topology definition file.

```yaml
topology:
  defaults:
    env-files:
      - envfiles/defaults
      - /home/user/clab/default-env
  kinds:
    nokia_srlinux:
      env-files:
        - envfiles/common
        - ~/spines
  nodes:
    node1:
      env-files:
        - /home/user/somefile
```

### user

To set a user which will be used to run a containerized process use the `user` configuration option. Can be defined at `node`, `kind` and `global` levels.

```yaml
topology:
  defaults:
    user: alice # alice user will be used for all nodes unless set on kind or node levels
  kinds:
    nokia_srlinux:
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
    nokia_srlinux:
      entrypoint: entrypoint.sh
  nodes:
    node1:
      entrypoint: entrypoint.sh
```

### cmd

It is possible to set/override the command of the container image with `cmd` configuration option. It accepts the "shell" form and can be set on all levels.

```yaml
topology:
  defaults:
    cmd: bash cmd.sh
  kinds:
    nokia_srlinux:
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
    nokia_srlinux:
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

!!!note
    Both user-defined and containerlab-assigned labels also promoted to environment variables prefixed with `CLAB_LABEL_` prefix.

### mgmt-ipv4

To make a node to boot with a user-specified management IPv4 address, the `mgmt-ipv4` setting can be used. Note, that the static management IP address should be part of the subnet that is used within the lab.

Read more about user-defined management addresses [here](network.md#user-defined-addresses).

```yaml
nodes:
    r1:
      kind: nokia_srlinux
      mgmt-ipv4: 172.20.20.100
```

### mgmt-ipv6

To make a node to boot with a user-specified management IPv4 address, the `mgmt-ipv6` setting can be used. Note, that the static management IP address should be part of the subnet that is used within the lab.

Read more about user-defined management addresses in the [networking guide](network.md#user-defined-addresses).

```yaml
nodes:
    r1:
      kind: nokia_srlinux
      mgmt-ipv6: 3fff:172:20:20::100
```

### DNS

To influence the DNS configuration a particular node uses, the `dns` configuration knob should be used. Within this blob, DNS server addresses, options and search domains can be provisioned.

```yaml
topology:
  nodes:
    r1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
      dns:
        servers:
          - 1.1.1.1
          - 8.8.4.4
        search:
          - foo.com
        options:
          - some-opt
```

### network-mode

By default containerlab nodes use bridge-mode driver - nodes are created with their first interface connected to a docker network (management network).

It is possible to change this behavior using `network-mode` property of a node.

#### host mode

The `network-mode` configuration option set to `host` will launch the node in the [host networking mode](https://docs.docker.com/network/host/).

```yaml
# example node definition with host networking mode
my-node:
  image: alpine:3
  network-mode: host
```

#### container mode

Additionally, a node can join network namespace of another container - by referencing the node in the format of `container:parent_node_name`[^2]:

```yaml
# example node definition with shared network namespace
my-node:
  kind: linux
sidecar-node:
  kind: linux
  network-mode: container:my-node # (1)
  startup-delay: 10 # (2)
```

1. `my-node` portion of a `network-mode` property instructs `sidecar-node` to join the network namespace of a `my-node`.
2. `startup-delay` is required in this case in order to properly initialize the namespace of a parent container.

Container name used after `container:` portion can refer to a node defined in containerlab topology or can refer to a name of a container that was launched outside of containerlab. This is useful when containerlab node needs to connect to a network namespace of a container deployed by 3rd party management tool (e.g. k8s kind).

#### none mode

If you want to completely disable the networking stack on a container, you can use the `none` network mode. In this mode containerlab will deploy nodes without `eth0` interface and docker networking. See [docker docs](https://docs.docker.com/network/none/) for more details.

### runtime

By default containerlab nodes will be started by `docker` container runtime. Besides that, containerlab has experimental support for `podman`, and `ignite` runtimes.

It is possible to specify a global runtime with a global `--runtime` flag, or set the runtime on a per-node basis:

Options for the runtime parameter are:

- `docker`
- `podman`
- `ignite`

The default runtime can also be influenced via the `CLAB_RUNTIME` environment variable, which takes the same values as mentioned above.

```yaml
# example node definition with per-node runtime definition
my-node:
  image: alpine:3
  runtime: podman
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

/// details | exec and access to env vars
When you want the `exec` command to have access to the env variables defined in the topology file or in the container' environment you have to escape the `$` sign:

```yaml
  nodes:
    test:
      kind: linux
      image: alpine:3
      env:
        FOO: BAR
      exec:
        - ash -c 'echo $$FOO'
```

///

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

### shm-size

The `shm-size` parameter can be used to customize the the shared memory size limit allocated to the container.
By default, this limit is 64MB with docker runtime.

```yaml
# my-node will be allocated 256MB of shared memory.
my-node:
  image: alpine:3
  kind: linux
  shm-size: 256MB
```

Supported memory suffixes (case insensitive): `b`, `kib`, `kb`, `mib`, `mb`, `gib`, `gb`.

### devices

The `devices` parameter can be used to add host devices to the container.

```yaml
# my-node will be able to access the host /dev/ppp and /dev/net/tun devices.
my-node:
  image: alpine:3
  kind: linux
  devices:
    - /dev/ppp
    - /dev/net/tun
```

### cap-add

The `cap-add` parameter can be used to add capabilities to the container.
Docker containers are currently executed in privileged mode, so this should not be needed.
If this becomes configurable, specifying the capabilities required for a container will be useful.

```yaml
# my-node will be given the NET_ADMIN and the SYS_ADMIN capabilities
my-node:
  image: alpine:3
  kind: linux
  cap-add:
    - NET_ADMIN
    - SYS_ADMIN
```

### sysctls

The sysctl container' setting can be set via the `sysctls` knob under the `defaults`, `kind` and `node` levels.

The sysctl values will be merged. Certain kinds already set up sysctl values in the background, which take precedence over the user-defined values.

The following is an example on how to setup the sysctls.

```yaml
topology:
  defaults:
    sysctls:
      net.ipv4.ip_forward: 1
      net.ipv6.icmp.ratelimi: 100
  kinds:
    nokia_srlinux:
      sysctls:
        net.ipv4.ip_forward: 0
        
  nodes:
    node1:
      sysctls:
        net.ipv6.icmp.ratelimit: 1000
```

### stages

Stages are a way to define stages a node goes through during its lifecycle and the interdependencies between the different stages of different nodes in the lab.

The stages are currently mainly used to host the `wait-for` knob, which is used to define the startup dependencies between nodes.

The following stages have been defined for a node:

- `create` - a node enters this stage when containerlab is about to create the node's container. The node finishes this stage when the container is created and is in the `created` state.
- `create-links` - a node enters this stage when containerlab is about to attach the links to the node. The node finishes this stage when all the links have been attached to the node.
- `configure` - a node enters this stage when containerlab is about to run post-deploy commands associated with the node. The node finishes this stage when post deployment commands have been completed.
- `healthy` - this stage has no distinctive enter/exit points. It is used to define a stage where a node is considered healthy. The healthiness of a node is defined by the healthcheck configuration of the node and the appropriate container status.
- `exit` - a node reaches the exit state when the container associated with the node has `exited` status.

Stages can be defined on the `defaults`, `kind` and `node` levels.

#### wait-for

For the explicit definition of interstage dependencies between the nodes, the `wait-for` knob under the `stages` level can be used.

In the example below node four nodes are defined with different stages and `wait-for` dependencies between the stages.

1. `node1` will enter the `create` stage only after `node2` has **finished** its `create` stage.
2. `node2` will enter its `create` stage only after `node3` has **finished** its `create-links` stage. This means that all the links associated with `node3` has been attached to the `node3` node.
3. `node3` will enter its `create` stage only after `node4` has been found `healthy`. This means that `node4` container must be healthy for `node3` to enter the creation stage.
4. `node4` doesn't "wait" for any of the nodes, but it defines its own healthcheck configuration.

```yaml
  nodes:
    node1:
      stages:
        create:
          wait-for:
            - node: node2
              stage: create

    node2:
      stages:
        create:
          wait-for:
            - node: node3
              stage: create-links

    node3:
      stages:
        create:
          wait-for:
            - node: node4
              stage: healthy

    node4:
      healthcheck:
        start-period: 5
        interval: 1
        test:
          - CMD-SHELL
          - cat /etc/os-release
```

Containerlab's built-in Dependency Manger takes care of all the dependencies, both explicitly-defined and implicit ones. It will inspect the dependency graph and make sure it is acyclic. The output of the Dependency Manager graph is visible in the debug mode.

Note, that `wait-for` is a list, a node's stage may depend on several other nodes' stages.

/// admonition | Usage scenarios
    type: tip
One of the use cases where `wait-for` might be crucial is when a number of VM-based nodes are deployed. Typically, simultaneous deployment of VMs might lead to shortage of CPU resources and VMs might fail to boot. In such cases, `wait-for` can be used to define the order of VM deployment, thus ensuring that certain VMs enter their `create` stage after certain nodes have reached `healthy` status.
///

#### Per-stage command execution

The existing [`exec`](#exec) node configuration parameter is used to run commands when then node has finished all its deployment stages. Whilst this is the most common use case, it has its limitations, namely you can't run commands when the node is about to deploy its links, or when it is about to enter the `healthy` stage.

These more advanced command execution scenarios are enabled in the per-stage command execution feature.

With per-stage command execution the user can define `exec` block under each stage; moreover, it is possible to specify when the commands should be run `on-enter` or `on-exit` of the stage. And if that is not enough, you can also specify where the command should be executed, on the host or in the container.

```yaml
nodes:
  node1:
    stages:
      create-links:
        exec:
          - command: ls /sys/class/net/
            target: container #(1)!
            phase: on-enter #(2)!
```

1. `target` defaults to "container" and can be omitted. Possible values `container` or `host`
2. `phase` defaults to "on-enter" and can be omitted. Possible values `on-enter` or `on-exit`

In the example above, the `ls /sys/class/net/` command will be executed when `node1` is about to enter the `create-links` stage. As expected, the command will list only interfaces provisioned by docker (eth0 and lo), but none of the containerlab-provisioned interfaces, since the create-links stage has not been finished yet.

Per-stage command execution gives you additional flexibility in terms of when the commands are executed, and what commands are executed at each stage.

##### Host exec

The stage's `exec` property runs the commands in the container namespace and therefore targets the container node itself. This is super useful in itself, but sometimes you need to run a command on the host as a reaction to a stage enter/exit event.

This is what `target` property of the stage's `exec` is designed for. It runs the command in the host namespace and therefore targets the host itself.

```yaml
nodes:
  node1:
    stages:
      create-links:
        exec:
          - command: touch /tmp/hello
            target: host
            phase: on-enter
```

In the example above, containerlab will run `touch /tmp/hello` command when the `node1` is about to enter the `create-links` stage.

### certificate

To automatically generate a TLS certificate for a node and sign it with the Certificate Authority created by containerlab, use `certificate.issue: true` parameter.  
The signed certificate will be stored in the [Lab directory](conf-artifacts.md#identifying-a-lab-directory) under the `.tls/<NODE_NAME>/` folder.

Note, that nodes which by default rely on TLS-enabled interfaces will generate a certificate regardless of this parameter.

```yaml
name: cert-gen

topology:
  nodes:
    a1:
      kind: linux
      image: alpine:latest
      certificate:
        issue: true
```

To configure key size and certificate validity duration use the following options:

```yaml
  certificate:
    issue: true
    key-size: 4096
    validity-duration: 1h
```

#### subject alternative names (SAN)

With `SANs` field of the certificate block the user sets the Subject Alternative Names that will be added to the node's certificate.

For a topology node named "srl" in a lab named "srl01", the following SANs are set by default:

- `srl`
- `clab-srl01-srl`
- `srl.srl01.io`
- IPv4/6 addresses of the node

```yaml
topology:

  nodes:
    srl:
      kind: nokia_srlinux
      certificate:
        sans:
          - "test.com"
          - 192.168.96.155
```

### healthcheck

Containerlab supports the [docker healthcheck](https://docs.docker.com/engine/reference/builder/#healthcheck) configuration for the nodes. The healthcheck instruction can be set on the `defaults`, `kind` or `node` level, with the node level likely being the most used one.

Healtcheck allows containerlab users to define the healthcheck configuration that will be used by the container runtime to check the health of the container.

```yaml
topology:
  nodes:
    l1:
      kind: linux
      image: alpine:3
      healthcheck:
        test:
          - CMD-SHELL
          - cat /etc/os-release
        start-period: 3
        retries: 1
        interval: 5
        timeout: 2
```

The healthcheck instruction is a dictionary that can contain the following keys:

- `test` - the command to run to check the health of the container. The command is provided as a list of strings. The first element of the list is the type of the command - either `CMD` or `CMD-SHELL`, the rest are the arguments.  
    When `CMD` type is used, the command and its arguments should be provided as a separate list elements. The `CMD-SHELL` allows you to specify the command that will be evaluated by the container's shell.
- `start-period` - the time in seconds to wait for the container to bootstrap before running the first health check. The default value is 0 seconds.
- `interval` - the time interval between the health checks. The default value is 30 seconds.
- `timeout` - the time to wait for a single health check operation to complete. The default value is 30 seconds.
- `retries` - the number of consecutive healthcheck failures needed to report the container as unhealthy. The default value is 3.

When the node is configured with a healthcheck the health status is visible in the `docker inspect` and `docker ps` outputs.

[^1]: [docker runtime resources constraints](https://docs.docker.com/config/containers/resource_constraints/).
[^2]: this deployment model makes two containers to use a shared network namespace, similar to a Kubernetes pod construct.

### aliases

To define additional hostnames for the node use the `aliases` configuration option. Other containers on the same network can use these aliases to communicate with the node.

```yaml
topology:
  nodes:
    r1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
      aliases:
        - r1.example.com
```
