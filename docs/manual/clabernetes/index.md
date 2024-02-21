---
status: new
---

# Clabernetes

<small>pronounciation: *Kla-ber-net-ees*</small>

Love containerlab? Want containerlab, just distributed in a kubernetes cluster? Enter [**clabernetes**](https://github.com/srl-labs/clabernetes/) or simply **c9s**.

![Clabernetes](https://gitlab.com/rdodin/pics/-/wikis/uploads/9d8c5abcb8db2c80811635d928aa98df/c9s_logo1_border_2.webp){ align=left width="300" }

<figure markdown>
![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/4fdd35b5f4553d766216a4bda2b9a20c/geogebra-export.svg#only-light)
![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/a139e454c70614298f5bf5b86fe1eeb0/geogebra-export-darkbg.svg#only-dark)
</figure>

Clabernetes deploys containerlab topologies into a kubernetes cluster. The goal of Clabernetes is to scale Containerlab beyond a single node while keeping the user experience you love.

If all goes to plan, Clabernetes is going to be one of the solutions to enable [multi-node labs](../multi-node.md) and allow its users to create large topologies powered by a k8s cluster.

Eager to try it out? Check out the [Quickstart](quickstart.md)! Have questions, join our [Discord](https://discord.gg/2A8ZxM7hD9).

/// warning
We are sharing Clabernetes β version to allow people to see what we're working on and potentially attract contributors and early adopters. You may not need any k8s knowledge to use it, but if something goes wrong, you might need to dig into k8s logs and resources to figure out what's happening.

In the beta release we focus on the core topology constructs working our way towards full feature parity with Containerlab (and even more).
///

## Quick Links

* [Helm chart on ArtifactHub](https://artifacthub.io/packages/helm/clabernetes/clabernetes)
* [CRD reference](https://doc.crds.dev/github.com/srl-labs/clabernetes)
* Source code on [GitHub](https://github.com/srl-labs/clabernetes)
