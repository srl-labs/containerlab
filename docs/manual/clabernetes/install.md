# Installation

Clabernetes controller (a.k.a manager) is installed via Helm; this means if you don't have Helm - [install it](https://helm.sh/docs/intro/install/), it's easy.

With Helm installed, to install the latest released Clabernetes to an existing k8s cluster[^1] do:

```bash
helm upgrade --install \
    clabernetes oci://ghcr.io/srl-labs/clabernetes/clabernetes
```

To upgrade to the latest version re-run the installation command and the latest version will be installed on the cluster.

To install a specific clabernetes version add `--version` flag like so:

```bash
helm upgrade --version 0.0.5 --install \
    clabernetes oci://ghcr.io/srl-labs/clabernetes/clabernetes
```

[^1]: Want to quickly spin up a local k8s clsuter with clabernetes? Check out our [Quickstart](quickstart.md).
