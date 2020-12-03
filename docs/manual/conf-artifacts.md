When containerlab deploys a lab it creates a Lab Directory in the **current working directory**. This directory is used to keep all the necessary files that are needed to run/configure the nodes. We call these files _configuration artifacts_.

Things like:

* Root CA certificate and per-node TLS certificate and private keys
* per-node config file
* node-specific files and directories that are required to launch the container
* license files

all these artifacts will be generated under a Lab Directory.

### Identifying a lab directory
The lab directory name will be constructed by the following template `clab-<lab_name>`. Thus if the name of your lab is `srl02` you will find the `clab-srl02` directory created after the lab deployment process is finished.

```
❯ ls -lah clab-srl02
total 4.0K
drwxr-xr-x  5 root root   40 Dec  1 22:11 .
drwxr-xr-x 23 root root 4.0K Dec  1 22:11 ..
drwxr-xr-x  5 root root   42 Dec  1 22:11 ca
drwxr-xr-x  3 root root   79 Dec  1 22:11 srl1
drwxr-xr-x  3 root root   79 Dec  1 22:11 srl2
```

The contents of this directory will contain kind-specific files and directories. Containerlab will name directories after the node names and will only created those if they are needed. For instance, by default any node of kind `linux` will not have it's own directory. 

### Persistance of a lab directory
When a user first deploy a lab, the Lab Directory gets created. Depending on node kind, this directory might act as a persistent storage area for a node. A common case is having the configuration file saved when the changes are made to the node via management interfaces.

Below is an example of the `srl1` node directory contents. It keeps a directory that is mounted to containers configuration path, as well as stores additional files needed to launch and configure the node.

```
~/clab/clab-srl02
❯ ls -lah srl1
drwxrwxrwx+ 6 1002 1002   87 Dec  1 22:11 config
-rw-r--r--  1 root root 2.8K Dec  1 22:11 license.key
-rw-r--r--  1 root root 4.4K Dec  1 22:11 srlinux.conf
-rw-r--r--  1 root root  233 Dec  1 22:11 topology.yml
```

When a user destroys a lab without providing [`--cleanup`](../cmd/destroy.md#cleanup) flag to the `destroy` command, the Lab Directory **does not** get deleted. This means that every configuration artefact will be kept on disk.

Moreover, when the user will deploy the same lab, containerlab will reuse the configuration artifacts if possible, which will, for example, start the nodes with the config files saved from the previous lab run.

To be able to deploy a lab without reusing existing configuration artefact use the [`--reconfigure`](../cmd/deploy.md#reconfigure) flag with `deploy` command. With that setting, containerlab will first delete the Lab Directory and then will start the deployment process.