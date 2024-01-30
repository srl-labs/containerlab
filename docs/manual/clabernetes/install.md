# Installation

Clabernetes runs on a Kubernetes cluster and hence requires one to be available before you start your Clabernetes journey. Although we don't have a strict requirement on the k8s version, we recommend using the version 1.21 or higher.

Clabernetes project consists of two components:

- Clabernetes manager (a.k.a. controller) - a k8s controller that watches for the Clabernetes topology resources and deploys them to the cluster.
- Clabverter - a CLI tool that converts containerlab topology files into Clabernetes topology resources.

## Clabernetes Manager

Clabernetes manager (a.k.a. controller) is packaged as a [Helm chart][chart-artifact]; this means if you don't have Helm - [install it](https://helm.sh/docs/intro/install/) or use it in a container packaging:

--8<-- "docs/manual/clabernetes/quickstart.md:helm-alias"

/// tab | install latest version
To install the latest Clabernetes release with Helm to an existing k8s cluster[^1] run the following command:
<!-- --8<-- [start:chart-install] -->
```bash
helm upgrade --install --create-namespace --namespace c9s \
    clabernetes oci://ghcr.io/srl-labs/clabernetes/clabernetes
```
<!-- --8<-- [end:chart-install] -->

To upgrade to the latest version re-run the installation command and the latest version will be installed on the cluster replacing the older running version.
///
/// tab | install specific version
To install a specific clabernetes version add `--version` flag like so:

```bash
helm upgrade --version 0.0.22 --install \
    clabernetes oci://ghcr.io/srl-labs/clabernetes/clabernetes
```

///
/// tab | uninstall
To uninstall clabernetes from the cluster:

```bash
helm uninstall clabernetes
```

///

## Clabverter

What a name, huh? Clabverter is a helper CLI tool that takes your existing containerlab topology converts it to a Clabernetes topology resource and applies it to the cluster.

Clabverter is versioned in the same way as Clabernetes, and the easiest way to use it is by leveraging the container image[^2]:

///tab | latest version
<!-- --8<-- [start:cv-install] -->
```bash title="set up <code>clabverter</code> alias"
alias clabverter="mkdir -p converted && chown -R 65532:65532 converted && \
    sudo docker run -v $(pwd):/clabernetes/work --rm \
    ghcr.io/srl-labs/clabernetes/clabverter"
```
<!-- --8<-- [end:cv-install] -->
///
///tab | specific version
In case you need to install a specific version:

```bash
alias clabverter="mkdir -p converted && chown -R 65532:65532 converted && \
    sudo docker run -v $(pwd):/clabernetes/work --rm \
    ghcr.io/srl-labs/clabernetes/clabverter:0.0.22"
```

///
///tab | development version
To use the latest development version of clabverter:

```bash
alias clabverter="mkdir -p converted && chown -R 65532:65532 converted && \
    sudo docker run -v $(pwd):/clabernetes/work --rm \
    ghcr.io/srl-labs/clabernetes/clabverter:dev-latest"
```

///
[chart-artifact]: https://artifacthub.io/packages/helm/clabernetes/clabernetes
[^1]: Want to quickly spin up a local k8s cluster with clabernetes? Check out our [Quickstart](quickstart.md).
[^2]: You already have Docker installed if you use containerlab, right?
