# Installation

## Controller

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

## Clabverter

What a name, huh? Clabverter is a helper CLI tool that tries to make your life easier when you want to make your existing containerlab topology to work in a k8s setting.

Clabverter is versioned the same way as Clabernetes, and the recommended way to use it is by leveraging the container image[^2] we offer:

=== "installing latest version"
    <!-- --8<-- [start:cv-install] -->
    ```bash
    docker pull ghcr.io/srl-labs/clabernetes/clabverter
    alias clabverter="docker run -v $(pwd):/clabverter --rm \
        ghcr.io/srl-labs/clabernetes/clabverter"
    ```
    <!-- --8<-- [end:cv-install] -->
=== "installing specific version"

    In case you need to install a specific version:

    ```bash
    alias clabverter="docker run -v $(pwd):/clabverter --rm \
        ghcr.io/srl-labs/clabernetes/clabverter:<version>"
    ```
=== "installing development version"
    In case you need to install a specific version:

    ```bash
    alias clabverter="docker run -v $(pwd):/clabverter --rm \
        ghcr.io/srl-labs/clabernetes/clabverter:dev-latest"
    ```

[^1]: Want to quickly spin up a local k8s clsuter with clabernetes? Check out our [Quickstart](quickstart.md).
[^2]: You anyway has Docker installed if you use containerlab, right?
