---
status: new
comments: true
---

# Containerlab labs in Codespaces

The best labs are the labs that you can run anywhere, anytime, with a single click and preferably for free.

Containerlab commoditized the labbing experience by providing a simple and easy to use tool to create and manage network topologies. But still you have to think a machine to run the lab on.  
Or, rather, you **had**.

We started to ship a [Dev Container][devcontainers-doc][^1] package for Containerlab that allows you to run containerlab-based labs in a [GitHub Codespaces][codespaces-doc] for free[^2] unlocking a whole new level of flexibility and convenience for users.

## Labs in Codespaces

[GitHub Codespaces][codespaces-doc] is a cloud-based development environment by GitHub that allows you to spin up a fully configured personal dev environment in the cloud and start coding in seconds. If you think about Containerlab as a Lab-as-Code solution, you can quickly see how these two can be a perfect match.

With Containerlab in Codespaces you can:

1. Spin up an existing lab with a single click without having to install anything on your local machine.
2. Start with Containerlab using the cloud IDE provided by Codespaces.

Here is a quick demo how anyone can run the full [SR Linux Streaming Telemetry lab][srl-telemetry-lab] by just [clicking on a link](https://codespaces.new/srl-labs/srl-telemetry-lab?quickstart=1). It is hard to imagine an easier way to run your labs in the cloud.

<video width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/b3e83eb56d674a0e967a74e020399e29/srl-tel-sneak.mp4" type="video/mp4">
</video>

<small>Fancy a full demo? Check out the 17min [:simple-youtube: video](https://www.youtube.com/watch?v=kpmTa9h0I-Q) by Roman. Would you rather straight try it yourself, [please go ahead](https://codespaces.new/srl-labs/srl-telemetry-lab?quickstart=1).</small>

### How does it work?

The key ingredients in this recipe are [GitHub codespaces][codespaces-doc] and the [Dev Container][devcontainers-doc] image that we provide for Containerlab. When a user clicks on a link[^3] to open the lab in Codespaces, GitHub spins up a Codespace environment and uses the Dev Container image to set it up. The [Containerlab' Dev Container image][clab-devcontainer] has all the necessary tools and dependencies to run Containerlab:

- containerlab binary
- shell configuration
- system packages and tools (gnmic, gnoic)
- VS Code plugins
- and anything else we consider useful to have in the environment

A user can choose to open the Codespace in a browser or in the local VS Code instance. In both cases, you get a fully functional environment with Containerlab installed and ready to use.

Codespaces environment boots for a couple of minutes, but once it is up and running, you see the familiar VS Code interface with the terminal window where you can run containerlab (and any other) commands.

### Codespaces

As we mentioned, the Codespaces environment is a VM in the cloud; you can install packages, run other workloads, and use the VM in any way you like, you have the full control of it. What makes Codespaces VM different from a any other VM in the cloud is that it is tightly integrated with GitHub and VS Code, and provides a configurable and ready-to-use environment.

We said it is free, but it is free to a certain extent, let's dig in.

#### Free plan

The best part about Codespaces is that it has a suitable free tier. GitHub offers **120 cpu-hours/month and 15 GB storage for free**[^4] to all users. This means that you can run a Codespace environment for 120 cpu-hours per month without any charges. This is a compelling offer for those who

- want to spin up a lab provided by others to get through a tutorial or a demo
- don't need to run the labs 24/7
- want to demo labs on-the-go without managing the local environment

You can select which GitHub machine type you want to use for your project; each machine type is characterized by the amount of CPUs/RAM/Storage that it is equipped with, and based on that you can calculate how many cpu-hours you would consume running a lab with a chosen machine type.
<!-- --8<-- [start:request-beefier] -->
<small>By the time of this writing (Jun 2024) the following machine types were available for GitHub users by default and beefier machines can be requested via [GitHub support form](https://support.github.com/contact?tags=rr-general-technical).</small>
<!-- --8<-- [end:request-beefier] -->

| Machine type | Memory (GB) | Storage (GB) | Run time included in the free tier[^5]<br>(hours/month) |
| ------------ | ----------- | ------------ | ------------------------------------------------------- |
| 2 core       | 4           | 32           | 60 (2cpu*60h=120 cpu/hours)                             |
| 4 core       | 8           | 32           | 30                                                      |
| 8 core       | 16          | 32           | 15                                                      |
| 16 core      | 32          | 64           | 7.5                                                     |
| 32 core      | 64          | 128          | 3.75 (may not be available in your account)             |

<small>If you need more than 120 cpu-hours, you can pay for the additional usage ([consult with pricing](https://github.com/features/codespaces#pricing)), and you can always stop the environment when you don't need it to save the quota.</small>

Your cpu-hours counter is reset at the beginning of each month, so you can use the free plan every month. And by default you have a $0 spending limit, so you won't be charged unless you explicitly allowed it. Good!

#### Control panel

Whenever you need to check what Codespaces environments you have running or created, you can do it in the [Codespaces panel][codespace-panel].

![panel](https://gitlab.com/rdodin/pics/-/wikis/uploads/73aa0c2154959d4b25f2b1c2dbd1380b/image.png){.img-shadow}

The panel allows you to see and interact with the available Codespaces environments, including starting, stopping, and deleting them. You can also check what repositories are associated with each environment which is useful for a Containerlab user to quickly identify the lab environments.

#### Codespaces settings

Codespaces expose a bunch of per-user settings at the [github.com/settings/codespaces](https://github.com/settings/codespaces) page. The following settings are worth mentioning:

//// define

Idle timeout and retention period

- Maybe the most important settings that you can configure in Codespaces. They allow you to control how long the environment will be running and when it will be deleted. Read more on this in [Billing](#billing) section

Secrets

- Secrets allow you to store sensitive information that will be available to your Codespaces environments. You can use them to store API keys, passwords, and other sensitive data that you don't want to expose in your code.

Setting Sync

- To make codespaces env feel like home, you can sync your settings across all your Codespaces environments. This includes themes, keybindings, and other settings that you have configured in your local VS Code instance.

Editor preference

- You can choose if you want to run the codespaces in a browser, in a local VS Code instance, or via a bridge to a JetBrains IDE.

////

#### Billing

It is always a good idea to periodically check how much of the cpu-hours you've consumed and check the remaining quota. Your billing information is available in the [Billing settings][billing].

![billing](https://gitlab.com/rdodin/pics/-/wikis/uploads/4bf6eb89bd3b4f2d4b05ecdc3ab22675/image.png){.img-shadow}

The screenshot shows that 10 cpu-hours out of 120 available were consumed in the current month' period and the codespaces environments occupy 8.15 GB of storage out of 15 GB included. So far it is all well within the free tier limits.[^6]

/// note
All users by default have a $0 spending limit[^7], which means that if you exceed the free tier limits, your environments will be stopped and you will **not** be charged. You can change this limit to a higher value if you want to be able to use Codespaces even after you exceed the free tier limits.
///

To avoid any surprises and lower your anxiety levels, GitHub Codespaces have two important settings that you configure at [github.com/settings/codespaces](https://github.com/settings/codespaces):

1. **[Idle timeout](https://docs.github.com/en/codespaces/setting-your-user-preferences/setting-your-timeout-period-for-github-codespaces)**  
    This setting allows you to "suspend" the running environment after a certain period of inactivity and defaults to 30 minutes. You can increase/decrease the timeout as you see fit. Consult with the docs to see what counts as activity and what doesn't.
2. **[Retention period](https://docs.github.com/en/codespaces/setting-your-user-preferences/configuring-automatic-deletion-of-your-codespaces)**  
    When you stopped the codespaces environment or it was suspended due to inactivity, it will be automatically deleted after a certain period of time. The default (and maximum) retention period is 30 days, but you can change it to a shorter period.  
    The stopped environment won't count against your cpu-hours quota, but it will still consume storage space, hence you might want to remove the stopped environments to free up the space.

/// admonition | Safe settings
    type: tip
To keep a tight control on the Codespaces free quota usage you can set the following in your [Codespaces Settings](https://github.com/settings/codespaces):

- Idle timeout to **15 minutes**
- Retention period to **1 day**

That way you can be sure that the environment is not running when you don't need it and it will be deleted after a day of inactivity saving up on the storage space.
///

## Adding codespaces to your lab

By now you should be willing to try running your labs in Codespaces. To our luck, it is super simple, all you need to do is create a `.devcontainer/devcontainer.json` file in your lab repository that will define the Codespaces environment. The file should look similar to this:

```json
{
    "image": "ghcr.io/srl-labs/containerlab/devcontainer-dind-slim:0.68.0",
    "hostRequirements": {
        "cpus": 4, // (1)!
        "memory": "8gb",
        "storage": "32gb"
    }
}
```

1. 4-core machine type is used in this example, you can tune the machine type to fit your lab requirements. Maybe it will fit in a 2-core/4GB machine, or you need a beefier 8-core machine, it is up to you.

<small>For a complete Dev Container specification, check out the [official docs](https://containers.dev/implementors/json_reference/).</small>

### Image

The `image` field points to the Containerlab Dev Container image that would define your Codespaces environment. Containerlab provides devcontainer images, and you can see all available tags on the [package' page][clab-devcontainer].

The image tag corresponds to the containerlab release version that is pre-installed in the image. You can choose the version that you want to use in your lab.

### Host requirements

Another important part of the `devcontainer.json` file is the `hostRequirements` field that defines the machine type that Codespaces environment will run on. Codespaces offer a small selection of machine types that differ in the number of CPUs, RAM, and storage. You can choose the machine type that fits your lab requirements.

--8<-- "docs/manual/codespaces.md:request-beefier"

| Machine type | CPU | Memory (GB) | Storage (GB) |
| ------------ | --- | ----------- | ------------ |
| 2 core       | 2   | 8           | 32           |
| 4 core       | 4   | 16          | 32           |
| 8 core       | 8   | 32          | 64           |
| 16 core      | 16  | 64          | 128          |

Using the machine types displayed above you can tune the `hostRequirements` section by choosing the machine type that fits the requirements of your lab.

/// note
Codespaces VMs support nested virtualization, so you can run [VM-based kinds](../manual/vrnetlab.md) :partying_face:
///

### Testing the environment

Once you added the `.devcontainer/devcontainer.json` file to your lab repository, you can test the environment locally and in Codespaces.

#### Local testing

Testing the environment locally is a litmus test to ensure that the `devcontainer.json` file is correct and the environment can be started. But since the environment runs locally, it doesn't test the Codespaces-specific settings like the machine type and CPU/RAM requirements.

To deploy the environment locally, make sure you have [Dev Containers VS Code extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) installed and then use the VS Code command panel (`Cmd/Ctrl+Shift+P`) to execute `Dev Containers: Rebuild And Reopen In Container` action. This will trigger the VS Code to build the container and open the environment in the container.

#### Remote testing

Once you tested the environment locally, you should test it in Codespaces to ensure that the selected machine type is sufficient for your lab.

Hopefully you've been adding the Codespaces support in a git branch and created a PR for it. You can open the PR in the GitHub UI and click on the "Code" -> "Create codespace on codespaces" button to start the environment for the branch you're working on:

![pr-codespaces](https://gitlab.com/rdodin/pics/-/wikis/uploads/915d9e48891da03f3f92f336e29b4829/image.png)

You'll get the environment up and running in a couple of minutes and you can test it to ensure that it works as expected.

### Launching the environment

Once you are satisfied with the environment, you can add a nice button to the README file that will allow users to start the environment with a single click.

/// tab | Button
<div align=center markdown>
<a href="https://codespaces.new/srl-labs/srlinux-vlan-handling-lab?quickstart=1">
<img src="https://gitlab.com/rdodin/pics/-/wikis/uploads/d78a6f9f6869b3ac3c286928dd52fa08/run_in_codespaces-v1.svg?sanitize=true" style="width:50%"/></a>

**[Run](https://codespaces.new/srl-labs/srlinux-vlan-handling-lab?quickstart=1) this lab in GitHub Codespaces for free**.  
[Learn more](https://containerlab.dev/manual/codespaces){data-proofer-ignore} about Containerlab for Codespaces.  
<small>Machine type: 2 vCPU · 8 GB RAM</small>
</div>
///
/// tab | Button code

The URL used in the link uses deep link configuration provided by Codespaces, read more about it in the [official docs](https://docs.github.com/en/codespaces/setting-up-your-project-for-codespaces/setting-up-your-repository/facilitating-quick-creation-and-resumption-of-codespaces).

**Do not forget to change the lab repo URL and machine type in the code below!**

```html
---
<div align=center markdown>
<a href="https://codespaces.new/srl-labs/srlinux-vlan-handling-lab?quickstart=1">
<img src="https://gitlab.com/rdodin/pics/-/wikis/uploads/d78a6f9f6869b3ac3c286928dd52fa08/run_in_codespaces-v1.svg?sanitize=true" style="width:50%"/></a>

**[Run](https://codespaces.new/srl-labs/srlinux-vlan-handling-lab?quickstart=1) this lab in GitHub Codespaces for free**.  
[Learn more](https://containerlab.dev/manual/codespaces) about Containerlab for Codespaces.  
<small>Machine type: 2 vCPU · 8 GB RAM</small>
</div>

---
```

///

Check out [srl-labs/srl-streaming-telemetry](https://github.com/srl-labs/srl-telemetry-lab?tab=readme-ov-file#nokia-sr-linux-streaming-telemetry-lab) README where this button is used to start the lab in Codespaces.

And of course, you can always launch the Codespace using the GitHub UI by clicking on the "Code" button.

![start](https://gitlab.com/rdodin/pics/-/wikis/uploads/34a9e417d61c02cf9a4283cc1c2cf8cb/image.png)

1. Click on the "Code" button in the GitHub UI.
2. Start the Codespace on the `main` branch using the devcontainer settings defined in the `.devcontainer/devcontainer.json` file.
3. Or open up an advanced menu
4. And configure the Codespace settings manually.

## Dev Container

The key pillar behind Codespaces is the Containerlab' Dev Container image that defines the environment in which the lab will run. The Dev Container image is a Docker image that contains all the necessary tools and dependencies to run Containerlab and other tools that you might need in the lab.

Containerlab has four devcontainer images that differ in the way the docker is setup and the tools installed (slim and regular variants):

1. Docker in Docker (dind) - is the devcontainer that is meant to contain an isolated docker environment inside the container. This image is **suitable for Codespaces**.
2. Docker outside of Docker (dood) - is a devcontainer image that mounts the docker socket from the outside, and therefore can reuse the images existing on the host machine. This image is mostly used with [DevPod](../macos.md#devpod).

You will find the devcontainer definition files in [containerlab/.devcontianer](https://github.com/srl-labs/containerlab/tree/main/.devcontainer) directory where:

1. devcontainer.json - the Dev Container configuration file that defines how the environment is built, configured and launched.
2. Dockerfile and slim.Dockerfile - the Dockerfile files that the Dev Container is built from.

The resulting Dev Container image (in a non-slim variant) contains the following tools and dependencies:

- containerlab binary installed via the deb repository
- docker in docker setup
- [gNMIc](https://gnmic.openconfig.net) and [gNOIc](https://gnoic.kmrd.dev) tools
- Go SDK
- Python 3 with `uv`
- `gh` CLI tool
- zsh shell with oh-my-zsh configuration
- VS Code plugins

## Catalog of Codespaces-enabled labs

Having a codespace-enabled lab makes it super easy for users to start the lab and get to the fun part of the labbing.

We encourage lab authors to add `codespaces` and `clab-topo` topics to the lab repository that supports Codespaces; that way users would be able to find the labs that they can run in Codespaces by following [**this link**](https://github.com/search?q=topic%3Aclab-topo+topic%3Acodespaces&type=repositories).

## Tips, tricks and known issues

### Authenticating with ghcr.io container registry

If you happen to have a _private_ image that you want to use in Codespaces you can push this image to your personal GitHub registry.

To be able to access a private image you would need to (re)authenticate with the `read:packages` token entitlement against the GitHub registry. Thankfully, it is a matter of a copy-paste exercise.

First, unset the existing token and request the one with `read:packages` capability:

```bash
unset GITHUB_TOKEN && gh auth refresh -s read:packages
```

You will be prompted to authenticate with your GitHub account and the new token will be generated for you. Then you can login to the registry using the newly acquired token:

```bash
gh auth token | \
docker login ghcr.io -u $(cat /home/vscode/.config/gh/hosts.yml | \
grep user: | awk '{print $2}') --password-stdin
```

<small>Of course you can also install tailscale or any other 0-tier VPN to access any other self-hosted private registry.</small>

[devcontainers-doc]: https://containers.dev/
[codespaces-doc]: https://github.com/features/codespaces
[clab-devcontainer]: https://github.com/srl-labs/containerlab/pkgs/container/containerlab%2Fdevcontainer-dind-slim
[billing]: https://github.com/settings/billing/summary
[codespace-panel]: https://github.com/codespaces
[srl-telemetry-lab]: https://github.com/srl-labs/srl-telemetry-lab

[^1]: Check out the [Dev Container section](#dev-container) to learn more about the Containerlab' Dev Container package.
[^2]: At the moment of writing, GitHub Codespaces offer 120 cpu-hours/month and 15 GB storage for free to all users. See [here](#free-plan) for more details.
[^3]: A link points to the codespaces environment and refers a repo with the `.devcontainer` folder that defines the environment. For example: https://codespaces.new/srl-labs/srl-telemetry-lab?quickstart=1
[^4]: The terms of the free plan may be subject to change, consult with the [official documentation](https://docs.github.com/en/billing/managing-billing-for-github-codespaces/about-billing-for-github-codespaces#monthly-included-storage-and-core-hours-for-personal-accounts) for the most recent information.
[^5]: The runtime assumes no other environments are running at the same time and storage quota is not exceeded.
[^6]: You can also see message about when the quota reset happens.
[^7]: As indicated by the "Monthly spending limit" text at the very bottom of the report table.
