For a traditional networking lab orchestration system `containerlab` appears to be quite unique in a way that it runs containers, not VMs. This inherently means that container images need to be available to spin up the nodes.

To keep things simple, containerlab adheres to the same principles of referencing container images as common tools like docker, podman, k8s do. The following example shows a clab file that references container images using various forms:

```yaml
name: images
topology:
  nodes:
    node1:
      # image from docker hub registry with implicit `latest` tag
      image: alpine
    node2:
      # image from docker hub with explicit tag
      image: ubuntu:20.04
    node3:
      # image from github registry
      image: ghcr.io/hellt/network-multitool
    node4:
      # image from some private registry
      image: myregistry.local/private/alpine:custom
```

When containerlab launches a lab, it reads the image name from the topology file and expects to find the referenced images locally or by pulling them from the registry.

If in the example above, the image named `myregistry.local/private/alpine:custom` was not loaded to docker local image store before, containerlab will attempt to pull this image and will expect the private registry to be reachable.

Container images offer a great flexibility and reproducibility of lab builds, to embrace it fully, we wanted to capture some basic image management operations and workflows in this article.

## Tagging images

A container image name can appear in various forms. A short form of `alpine` will be expanded by docker daemon to `docker.io/alpine:latest`. At the same time an image named `myregistry.local/private/alpine:custom` is already a fully qualified name and indicates the container registry (`myregistry.local`) image repository name (`private/alpine`) and its tag (`custom`).

With a [`docker tag`](https://docs.docker.com/engine/reference/commandline/tag/) command it is possible to "rename" an image to something else. This can be needed for various purposes, but most common needs are:

1. rename the image so it can be pushed to another repository
2. rename the image to users liking

Let's imagine that we have a private repository from which we pulled the image with a name `registry.srlinux.dev/pub/vr-sros:20.10.R3`. By using this name in our clab file we can make use of this image in our lab. But that is quite a lengthy name, we might want to shorten it to something less verbose:

```bash
# docker tag <old-name> <new-name>
docker tag registry.srlinux.dev/pub/vr-sros:20.10.R3 sros:20.10.R3
```

With that we make a new image named `sros:20.10.R3` that references the same original image. Now we can use the short name in our clab files.

### Pushing to a new registry

That same `docker tag` command can be used to rename the image so it can be pushed to another registry. For example consider the newly built SR OS 21.2.R1 [vrnetlab](vrnetlab.md) image that by default will have a name of `vrnetlab/vr-sros:21.2.R1`. This container image can't be pushed anywhere in its current form, but retagging will help us out.

If we wanted to push this image to a public registry like the Github Container Registry, we could do the following:

```bash
# retag the image to a fully qualified name that is suitable for
# push to github container registry
sudo docker tag vrnetlab/vr-sros:21.2.R1 ghcr.io/srl-labs/vr-sros:21.2.R1

# and now we can push it
sudo docker push ghcr.io/srl-labs/vr-sros:21.2.R1
```

## Exchanging images

Container images are a perfect fit for sharing. Once anyone built an image with a certain NOS inside it can share it with anyone via container registry. Sensitive and proprietary images are typically pushed to private registries and internal users pull it from there.

But sometimes you need to share an image with a colleague or your own setup that doesn't have access to a private registry. There are couple of ways to achieve that.

### As zipped tar archive

A container image can be saved as `tar.gz` file that you can then share via various channels:

```
sudo docker save vrnetlab/vr-sros:21.2.R1 | xz -T 0 > sros.tar.gz
```

Now you can push the tar.gz file to Google Drive, Dropbox, etc.

On the receiving end you can load the container image:

```
sudo docker load -i sros.tar.gz
```

### Via temp registry

Another cool way of sharing a container image is via [ttl.sh](https://ttl.sh) registry which offers a way to push an image to their public registry but the image will expire with a timeout you set.

For example, let's push our image to the ttl.sh registry under a random name and make it expire in 15 minutes.

```bash
# generate random 6 char sequence
IMAGE=$(cat /dev/urandom | tr -dc 'a-z0-9' | fold -w 6 | head -n 1)
# set ttl
TTL=15m

# tag and push
sudo docker tag vrnetlab/vr-sros:21.2.R1 ttl.sh/$IMAGE:$TTL
sudo docker push ttl.sh/$IMAGE:$TTL
echo "pull the image with \"docker pull ttl.sh/$IMAGE:$TTL\" in the next $TTL"
```

That is a very convenient way of sharing images with a small security compromise.
