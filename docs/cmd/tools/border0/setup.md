# border0 setup

### Description

The `setup` sub-command under the `tools border0` command provisions a Border0 organization with resources to expose a new ContainerLab environment over the Border0 service.

### Usage

`clab tools border0 setup [local-flags]`

### Flags

#### lab-name

The `--lab-name | -l` flag allows providing a name to associate with all resources created in Border0.

### Examples

```bash
clab tools border0 setup

Please navigate to the URL below in order to complete the login process:
https://portal.border0.com/login?device_identifier=IjdhYmEwYWEwLTNkYT...HhWK5aMnmxtZNDc
Login successful!

New lab initialized with the Border0 service 🚀
Add the following configuration to your *.clab.yaml file:

    border0:
      kind: linux
      image: ghcr.io/borderzero/border0
      cmd: connector start --config /etc/border0/border0.yaml
      binds:
        - /var/run/docker.sock:/var/run/docker.sock
        - /etc/border0/border-clab-9650-config.yaml:/etc/border0/border0.yaml


Once you deploy your ContainerLab enviroment, your containers will be available at:
https://client.border0.com/#/ssh/border-clab-9650-containers-somerandomname.border0.io

```