Nodes configuration is one of pillars of containerlab. By exposing nodes configuration we allow users to create flexible and customizable labs.

The node configuration **may** consist of the following fields that we explain in details below. The only mandatory node's configuration field is `kind`.

```yaml
# part of topology definition file
topology:
  nodes:
    node1:  # node name
      kind: srl
      type: ixrd2
      image: srlinux
      license: license.key
      config: /root/mylab/node1.cfg
      binds:
        - /usr/local/bin/gobgp:/root/gobgp
        - /root/files:/root/files:ro
```
### kind
The `kind` property selects which kind this node is of. Kinds are essentially a way of telling containerlab how to treat the nodes properties considering the specific flavor of the node. We dedicated a [separate section](kinds.md) to discuss kinds in details.

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
Some containerized NOSes require a license to operate. With `license` property a user sets a path to a license file that a node will use. The license file will then be mounted to the container by the path that is defined by the `kind/type` of the node.

### config
For the specific kinds its possible to pass a path to a config template file that a node will use.

The template engine is [Go template](https://golang.org/pkg/text/template/). The [srlconfig.tpl](https://github.com/srl-wim/container-lab/blob/master/templates/srl/srlconfig.tpl) template is used by default for Nokia SR Linux nodes and can be used to create configuration templates for SR Linux nodes.

Supported for: Nokia SR Linux.

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