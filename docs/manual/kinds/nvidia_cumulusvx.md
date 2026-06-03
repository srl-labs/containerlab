---
search:
  boost: 4
kind_code_name: nvidia_cumulusvx
kind_display_name: NVIDIA Cumulus VX
---
# -{{ kind_display_name }}-

[-{{ kind_display_name }}-](https://docs.nvidia.com/networking-ethernet-software/cumulus-vx/) is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).
The `-{{ kind_code_name }}-` kind defines a supported feature set and a startup procedure of a -{{ kind_display_name }}- node.

/// note

1. NVIDIA Cumulus VX as a standalone image [is not downloadable anymore](https://docs.nvidia.com/networking-ethernet-software/cumulus-vx/). However, if you get your hands on the image, you can use it with Containerlab.
2. The original container-based `cvx` kind (while still available in Containerlab) is not maintained/supported by the Containerlab team or community. Users will have to hunt for a suitable image and use this `-{{ kind_code_name }}-` kind instead.
///

## Building the image

The containerlab-compatible image can be built from the original qcow2 image file using experimental support in [srl-labs/vrnetlab](https://github.com/srl-labs/vrnetlab/pull/462) or [boxen-v2](https://github.com/carlmontanari/boxen/tree/feat/boxen2) projects.

## Managing -{{ kind_code_name }}- nodes

Cumulus VX node launched with containerlab can be managed via the following interfaces:

/// tab | SSH
SSH server is running on port 22

```bash
ssh cumulust@<container-name>
```

///
/// tab | REST API
NVUE REST API is running on port 8765

```bash
curl -k -u cumulus:Clab123! https://<container-name>:8765/nvue_v1/system
```

///
/// tab | bash
to attach to a `bash` shell of a running cvx container (only container ID is supported):

```bash
docker attach <container-id>
```

///

### Credentials

Username: `cumulus`  
Password: `Clab123!`

## Interface naming

Dataplane interfaces in your topology file should be named as `swpN`, where `N` is the port number and matches the port name in the network OS configuration.
