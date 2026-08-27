# Installation

Clabernetes runs on a Kubernetes cluster and hence requires one to be available before you start your Clabernetes journey. c9s 0.7 and newer require Kubernetes 1.31 or higher for the field selectors used by the Link API.

Clabernetes project consists of two main components:

- Clabernetes manager (a.k.a. controller) - k8s controllers that reconcile c9s `Node`, `Link`, `NodeProfile`, and `Topology` resources.
- Clabverter - a CLI tool that converts containerlab topology files into Clabernetes resources.

/// note | Using the containerlab runtime
When you use [`containerlab --runtime c9s`](runtime.md), containerlab
creates the `Topology` custom resource for you (or, with `--no-topology-cr`,
the primary `Node`, `Link`, and `NodeProfile` resources directly). In that
workflow you still need the Clabernetes manager and CRDs installed in the
cluster, but you don't need to run `clabverter` for every deployment.
///

## Clabernetes Manager

Clabernetes manager (a.k.a. controller) is packaged as a [Helm chart][chart-artifact]; this means if you don't have Helm - [install it](https://helm.sh/docs/intro/install/) or use it in a container packaging:

--8<-- "docs/manual/clabernetes/quickstart.md:helm-alias"

With Helm installed, proceed to install the Clabernetes manager.

/// tab | install latest version
To install the latest Clabernetes release with Helm to an existing k8s cluster[^1] run the following command:
<!-- --8<-- [start:chart-install] -->
```bash
helm upgrade --install --create-namespace --namespace c9s \
    clabernetes oci://ghcr.io/clabernetes/clabernetes/clabernetes
```
<!-- --8<-- [end:chart-install] -->

To upgrade to the latest version re-run the installation command and the latest version will be installed on the cluster replacing the older running version.
///
/// tab | install specific version
To install a specific clabernetes version add `--version` flag like so:

```bash
helm upgrade --version 0.0.25 --install \
    clabernetes oci://ghcr.io/clabernetes/clabernetes/clabernetes
```

///
/// tab | latest dev version
Clabernetes iterates fast, and you might want to try the latest development version until we cut a release. To do so, use the `0.0.0` version:

```bash
helm upgrade --install --version 0.0.0 --create-namespace --namespace c9s \
    --set manager.managerLogLevel=debug \
    --set manager.controllerLogLevel=debug \
    --set manager.imagePullPolicy=Always \
    clabernetes oci://ghcr.io/clabernetes/clabernetes/clabernetes
```

We also set the log level to `debug` for all the components to see more verbose logs. Trust us, you might need it :smile:
///
/// tab | uninstall
To uninstall clabernetes from the cluster:

```bash
helm uninstall --namespace c9s clabernetes
```

///

## Clabverter

What a name, huh? Clabverter is a helper CLI tool that takes your existing containerlab topology converts it to a Clabernetes topology resource and applies it to the cluster.

Clabverter is versioned in the same way as Clabernetes, and the easiest way to use it is by leveraging the container image[^2]:

///tab | latest version
<!-- --8<-- [start:cv-install] -->
```bash title="set up <code>clabverter</code> alias"
alias clabverter='sudo docker run --user $(id -u) \
    -v $(pwd):/clabernetes/work --rm \
    ghcr.io/clabernetes/clabernetes/clabverter'
```
<!-- --8<-- [end:cv-install] -->
///
///tab | specific version
In case you need to install a specific version:

```bash
alias clabverter='sudo docker run --user $(id -u) \
    -v $(pwd):/clabernetes/work --rm \
    ghcr.io/clabernetes/clabernetes/clabverter:0.7.0'
```

///
///tab | development version
To use the latest development version of clabverter:

```bash
alias clabverter='sudo docker run --pull always --user $(id -u) \
    -v $(pwd):/clabernetes/work --rm \
    ghcr.io/clabernetes/clabernetes/clabverter:dev-latest'
```

///
[chart-artifact]: https://artifacthub.io/packages/helm/clabernetes/clabernetes
[^1]: Want to quickly spin up a local k8s cluster with clabernetes? Check out our [Quickstart](quickstart.md).
[^2]: You already have Docker installed if you use containerlab, right?
