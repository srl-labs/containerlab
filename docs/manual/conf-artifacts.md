When containerlab deploys a lab it creates a Lab Directory in the current working directory. This directory is used to keep all the necessary files that are needed to run/configure the nodes. We call these files _configuration artifacts_.

Things like:

* Root CA certificate and per-node TLS certificate and private keys
* per-node config file
* node-specific files and directories that are required to launch the container
* license files

all these artifacts will be generated under a Lab Directory.

### Identifying a lab directory
The lab directory name will be constructed by the following template `clab-<lab_name>`. Thus if the name of your lab is `myAwesomeLab` you will find the `clab-myAwesomeLab` directory created after the lab deployment process is done.

The contents of this directory will contain kind-specific files and directories.

### Persistance of a lab directory
When a user first deploy a lab, the Lab Directory gets created. Depending on node kind, this directory might act as a persistent storage area for a node. A common case is having the configuration file saved when the changes are made to the node via management interfaces.

When a user destroys a lab without providing [`--cleanup`](../cmd/destroy.md#cleanup) flag to the `destroy` command, the Lab Directory **does not** get deleted. This means that every configuration artefact will be kept on disk.

Moreover, when the user will deploy the same lab, containerlab will reuse the configuration artifacts if possible, which will, for example, start the nodes with the config files saved from the previous lab run.

To be able to deploy a lab without reusing existing configuration artefact use the [`--regenerate`](../cmd/deploy.md#regenerate) flag with `deploy` command. With that setting, containerlab will first delete the Lab Directory and then will start the deployment process.