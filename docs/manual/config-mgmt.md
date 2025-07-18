# Configuration Management

Containerlab's prime task is to enable its user with a simple and intuitive interface to build, manage and share networking labs. And one of the important aspects of any lab is the configuration of its nodes.

Today, Containerlab offers the following ways to configure your lab nodes:

- [**Startup configuration file**](nodes.md#startup-config)  
    The most straight forward way to configure your nodes is to use the startup configuration file, that can be templated either prior the deployment phase, or during it.

    The config can be provided in the full canonical configuration format or by providing a file with commands that will be run on top of the default configuration.

- [**Ansible**](inventory.md#ansible)  
    Thanks to the auto-generated Ansible inventory, you can use any Ansible playbook to configure your nodes post deployment.

- [**Nornir**](inventory.md#nornir)  
    Containerlab also generates a simple Nornir YAML inventory, that can be used to configure your nodes post deployment.

## Startup configuration file

It is possible to provide the startup configuration that the node applies on boot for most Containerlab kinds. The startup config can be provided in two ways:

1. As a path to a file that is available on the host machine and contains the config blob that the node understands.
2. As an embedded config blob that is provided as a multiline string in YAML.
3. As a remote file using a URL that points to a file with the config blob.

### Local

When a path to a startup-config file is provided, containerlab either mounts the file to the container by a path that NOS expects to have its startup-config file, or it will apply the config via using the NOS-dependent method.

```yaml
topology:
  nodes:
    srl:
      startup-config: ./some/path/to/startup-config.cfg
```

Check the particular kind documentation to see if the startup-config is supported and how it is applied.

/// details | Startup-config path variable
By default, the startup-config references are either provided as an absolute or a relative (to the current working dir) path to the file to be used.

Consider a two-node lab `mylab.clab.yml` with seed configs that the user may wish to use in their lab. A user could create a directory for such files similar to this:

```
.
├── cfgs
│   ├── node1.partial.cfg
│   └── node2.partial.cfg
└── mylab.clab.yml

2 directories, 3 files
```

Then to leverage these configs, the node could be configured with startup-config references like this:

```yaml
name: mylab
topology:
  nodes:
    node1:
      startup-config: cfgs/node1.partial.cfg
    node2:
      startup-config: cfgs/node2.partial.cfg
```

while this configuration is correct, it might be considered verbose as the number of nodes grows. To remove this verbosity, the users can use a special variable `__clabNodeName__` in their startup-config paths. This variable will expand to the node-name for the parent node that the startup-config reference falls under.

```yaml
name: mylab
topology:
  nodes:
    node1:
      startup-config: cfg/__clabNodeName__.partial.cfg
    node2:
      startup-config: cfgs/__clabNodeName__.partial.cfg
```

The `__clabNodeName__` variable can also be used in the kind and default sections of the config.  Using the same directory structure from the example above, the following shows how to use the magic variable for a kind.

```yaml
name: mylab
topology:
  defaults:
    kind: nokia_srlinux
  kinds:
    nokia_srlinux:
      startup-config: cfgs/__clabNodeName__.partial.cfg
  nodes:
    node1:
    node2:
```

The following example shows how one would do it using defaults.

```yaml
name: mylab
topology:
  defaults:
    kind: nokia_srlinux
    startup-config: cfgs/__clabNodeName__.partial.cfg
  nodes:
    node1:
    node2:
```

///

### Embedded

It is possible to embed the startup configuration in the topology file itself. This is done by providing the startup-config as a multiline string.

```yaml
topology:
  nodes:
    srl:
      startup-config: |
        system information location "I am an embedded config"
```

/// admonition | Note
    type: subtle-note
If a config file exists in the lab directory for a given node, then it will take preference over the startup config passed with this setting. If it is desired to discard the previously saved config and use the startup config instead, use the `enforce-startup-config` setting or deploy a lab with the [`reconfigure`](../cmd/deploy.md#reconfigure) flag.
///

### Remote

It is possible to specify a remote (`http(s)` or [S3](s3-usage-example.md)) location for a startup-config file. Simply provide a URL that can be accessed from the containerlab host.

```yaml
topology:
  kinds:
    nokia_srlinux:
      type: ixrd3
      image: ghcr.io/nokia/srlinux
      startup-config: https://raw.githubusercontent.com/srl-labs/containerlab/main/tests/02-basic-srl/srl2-startup.cli
```

The remote file will be downloaded to the containerlab's temp directory at `$TMP/.clab/<filename>` path and provided to the node as a locally available startup-config file. The filename will have a generated name that follows the pattern `<lab-name>-<node-name>-<filename-from-url>`, where `<filename-from-url>` is the last element of the URL path.

/// admonition | Note
    type: subtle-note

- Upon deletion of a lab, the downloaded startup-config files will not be removed. A manual cleanup should be performed if required.
- If a lab is redeployed with the lab name and startup-config paths unchanged, the local file will be overwritten.
- For https locations the certificates won't be verified to allow fetching artifacts from servers with self-signed certificates.
///

### Customisation options

While many labs would be just fine with providing the partial or full configs to the lab nodes, some advanced labs might want to customize the startup config file before providing it to the node.

Containerlab offers a few options to customize the startup config - env vars expansion and templating.

#### Env vars expansion

Every startup configuration file that is provided to a lab node undergoes environment variable expansion procedure. This means you can embed environment variables in your startup config file and they will be expanded at runtime.

Both env vars defined in the Containerlab topology file and available in the host environment will be expanded. You can use advanced environment variable syntax to support default values, as shown in the [Environment variables](topo-def-file.md#environment-variables) documentation.

Here is an example of a lab that makes use of env vars expansion:

/// tab | Topology

```yaml
name: srl
topology:
  nodes:
    srl1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:25.3.1
      startup-config: srl-cfg-with-env.cfg
```

///
/// tab | Startup config

```
set / system information location ${SRL_LOCATION:=default-value}
```

///

If you deploy this lab with `SRL_LOCATION` env var set to some value, like shown below, you will have this value set to the provided value (or defaulted to `default-value` if the env var is not set).

```
SRL_LOCATION=Amsterdam containerlab deploy -t srl.clab.yaml
```

#### Templating

Env vars expansion is useful when your parametrization tasks are simple and limited to direct string substitution.

For more advanced config management tasks, you can use templating. Containerlab uses Go [text/template](https://pkg.go.dev/text/template)[^1] package for templating and users can provide a template as a startup config that will be rendered at runtime.

> The templating happens after env vars expansion, so you can use env vars in your template and templating together.

The templating engine receive the whole Node object as a template context. The [Node object](https://github.com/srl-labs/containerlab/blob/596f7e27b253e4d3039a17c694810bb90a650788/types/node_definition.go#L16) is everything you have defined under the `.topology.nodes.<node-name>` with the [defaults](topo-def-file.md#defaults) and [kinds](topo-def-file.md#kinds) merged in.

Here is an example how you can access values defined in your node definition from within the template:

/// tab | Topology

```yaml
name: srl
topology:
  nodes:
    srl1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:25.3.1
      startup-config: srl-cfg-with-template.cfg
```

///
/// tab | Startup config

Here we are referring to the `Image` field of the node object that is defined in the [NodeDefinition](https://github.com/srl-labs/containerlab/blob/596f7e27b253e4d3039a17c694810bb90a650788/types/node_definition.go#L27) struct.

```
set / system information location "I have been deployed from the image: {{ .Image }}"
```

///

Deploying this topology will result in the `location` string to return:

```
docker exec clab-srl-srl1 sr_cli info system information 
    location "I have been deployed from ghcr.io/nokia/srlinux:25.3.1"
```

As you can see, the templating engine accessed the `Image` field of the NodeDefinition struct and replaced it with the actual image name provided in the topology file.  

This was just a simple example to introduce you to the templating engine and the way it accesses templating data. Now let's see how you can make it work on a more complex example.

You can provide arbitrary variables data to your lab nodes by using the `.topology.nodes.<node-name>.config.vars` field, for example:

```yaml
name: srl
topology:
  nodes:
    srl1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:25.3.1
      config:
        vars:
          ifaces:
            - 2
            - 4
```

In the topology file above we defined a variable called `ifaces` that contains a list of interfaces to enable on the SR Linux node. Now we can create a template for the startup config that will enable the interfaces defined in the `ifaces` variable.

```go
{{- $ifaces := index .Config.Vars "ifaces" }}
{{- if $ifaces }}
{{- range $iface := $ifaces }}
set / interface ethernet-1/{{ $iface }} description "Interface {{ $iface }} configured by template"
{{- end }}
{{- end }}
```

With this template in place, deploy the lab and check the descriptions of the interfaces we configured via the template:

```
docker exec clab-srl-srl1 sr_cli info interface ethernet-1/\{2,4\} description
interface ethernet-1/2 {
    description "Interface 2 configured by template"
}
interface ethernet-1/4 {
    description "Interface 4 configured by template"
}
```

Note, that the `NodeDefinition` structure only defines `.Config.Vars` field, therefore to access any nested data structures you need to use the `index` function. Like in the example above where we access the `ifaces` list by using `index .Config.Vars "ifaces"`.

##### Functions

Go [text/template](https://pkg.go.dev/text/template) has built-in functions you can use in your template such as `range`, `index` and so on.

On top of the built-in function, Containerlab bundles the following custom functions:

- `strings.Split` - splits a string by a delimiter. Documented [here](https://docs.gomplate.ca/functions/strings/#stringssplit).
- `strings.ReplaceAll` - replaces all occurrences of a substring with a new substring. Documented [here](https://docs.gomplate.ca/functions/strings/#stringsreplaceall).
- `conv.Join` - joins a list of strings into a single string. Documented [here](https://docs.gomplate.ca/functions/conv/#convjoin).
- `conv.ToInt` - converts a string to an integer. Documented [here](https://docs.gomplate.ca/functions/conv/#convtoint).

> If a function you need is not available, check if it is available in the [gomplate](https://docs.gomplate.ca/functions/) documentation and ask a request to add it via Containerlab's issue tracker.

[^1]: You can use https://repeatit.io/ to play online with Go templating language.
