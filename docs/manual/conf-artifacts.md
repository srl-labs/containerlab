# Lab Directory and Configuration Artifacts

When containerlab deploys a lab it creates a Lab Directory in the same directory where your topology (`clab.yml`) file is. This directory is used to keep all the necessary files that are needed to run/configure the nodes. We call these files _configuration artifacts_ and/or lab state files.

Things like:

* CA certificate and node' TLS certificate and private keys
* node config file (if applicable and supported by the kind)
* node-specific files and directories that are required to launch the container
* license files if needed

All these artifacts will be available under a Lab Directory.

## Identifying a Lab Directory

The lab directory name follows the `clab-<lab_name>` template. Thus, if the name of your lab is `srl02` you will find the `clab-srl02` directory created by default in the directory where topology file is located. The location can be changed by setting the [`CLAB_LABDIR_BASE`](../cmd/deploy.md#clab_labdir_base) environment variable.

```
❯ ls -lah clab-srl02
total 4.0K
drwxr-xr-x  5 root root   40 Dec  1 22:11 .
drwxr-xr-x 23 root root 4.0K Dec  1 22:11 ..
drwxr-xr-x  5 root root   42 Dec  1 22:11 .tls
drwxr-xr-x  3 root root   79 Dec  1 22:11 srl1
drwxr-xr-x  3 root root   79 Dec  1 22:11 srl2
```

The contents of this directory will contain kind-specific files and directories. Containerlab will name directories after the node names and will only created those if they are needed. For instance, by default any node of kind `linux` will not have it's own directory under the Lab Directory.

### Persistence of a Lab Directory

When a user first deploy a lab, the Lab Directory gets created if it was not present. Depending on a node's kind, this directory might act as a persistent storage area for a node. A common case is having the configuration file saved when the changes are made to the node via management interfaces.

Below is an example of the `srl1` node directory contents. It keeps a directory that is mounted to containers configuration path, as well as stores additional files needed to launch and configure the node.

```
~/clab/clab-srl02
❯ ls -lah srl1
drwxrwxrwx+ 6 1002 1002   87 Dec  1 22:11 config
-rw-r--r--  1 root root 2.8K Dec  1 22:11 license.key
-rw-r--r--  1 root root 4.4K Dec  1 22:11 srlinux.conf
-rw-r--r--  1 root root  233 Dec  1 22:11 topology.clab.yml
```

When a user destroys a lab without providing the [`--cleanup`](../cmd/destroy.md#cleanup) flag to the `destroy` command, the Lab Directory **does not** get deleted. This means that every configuration artifact will be kept on disk.

Moreover, when the user will deploy the same lab, containerlab will reuse the configuration artifacts if possible, which will, for example, start the nodes with the config files saved from the previous lab run.

To be able to deploy a lab without reusing existing configuration artifact use the [`redeploy`](../cmd/redeploy.md) command with `--cleanup` or add [`--reconfigure`](../cmd/deploy.md#reconfigure) flag to the `deploy` command. With that setting, containerlab will first delete the Lab Directory and then will start the deployment process.
