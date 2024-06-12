---
status: new
---

# Containerlab labs in Codespaces

The best labs are the labs that you can run anywhere, anytime, with a single click and preferrably for free.

Containerlab commoditized the labbing experience by providing a simple and easy to use tool to create and manage network topologies. But still you have to have a machine to run it on.  
Or, rather, you **had**.

We started to ship containerlab in a [Dev Container][devcontainers-doc][^1] package that allows you to run containerlab in a [GitHub Codespaces][codespaces-doc] for free[^2] and this unlocks a whole new level of flexibility and convenience for the users.

## Labs in Codespaces

[GitHub Codespaces][codespaces-doc] is a cloud-based development environment by GitHub that allows you to spin up a fully configured personal dev environment in the cloud and start coding in seconds. If you think about Containerlab as a Lab-as-Code solution, you can quickly see how these two can be a perfect match.

With Containerlab in Codespaces you can:

1. Spin up an existing lab with a single click without having to install anything on your local machine.
2. Start with Containerlab from scratch using the cloud IDE provided by Codespaces.

Here is a quick demo how anyone can run the full SR Linux Streaming Telemetry lab by just clicking on a link. It is hard to imagine a more easy and convenient way to run your labs in the cloud.

<video width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/b3e83eb56d674a0e967a74e020399e29/srl-tel-sneak.mp4" type="video/mp4">
</video>

<small>Want to try it yourself? [Click here](https://codespaces.new/srl-labs/srl-telemetry-lab?quickstart=1).</small>

### How does it work?

The key ingredients in this recipe are [GitHub codespaces][codespaces-doc] and the [Dev Container][devcontainers-doc] image that we provide for Containerlab. When a GitHub user clicks on a link[^3] to open the lab in Codespaces, GitHub spins up a Codespace environment and uses the Dev Container image to set up the environment. The [Containerlab' Dev Container image][clab-devcontainer] contains all the necessary tools and dependencies to run Containerlab:

- containerlab binary
- shell configuration
- system packages and tools (gnmic, gnoic)
- VS Code plugins
- and anything else we consider useful to have in the environment

A user can choose to open the Codespace in a browser or in the local VS Code instance. In both cases, the user gets a fully functional environment with Containerlab installed and ready to use.

It takes a couple of minutes to set up and start the environment, but once it is up and running, you see the familiar VS Code interface with the terminal window where you can run containerlab (and any other) commands.

### Codespaces

As we mentioned, the Codepsaces environment is a VM in the cloud, you can install packages, run other workloads, and use the VM in any way you like, you have the full control of it. We mentioned it is free, but it is free to a certain extent, let's dig in.

#### Free plan

The best part about Codespaces is that it has a suitable free tier. GitHub offers **120 cpu-hours/month and 15 GB storage for free** to all users. This means that you can run a Codespace environment for 120 cpu-hours per month without any charges. This is a compelling offer for those who don't need to run the labs 24/7.

You can select which GitHub machine you want to use for your project; each machine type is characterized by the amount of CPUs/RAM/Storage that it is equipped with, and based on that you can calculate how many cpu-hours you would consume with a given machine type.

![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/fc7bfa89c921f965ce61f6eb5e86f312/image.png){.img-shadow}

<small>If you need more than 120 cpu-hours, you can pay for the additional hours, but the cost is reasonable, and you can always stop the environment when you don't need it to save the hours.</small>

Your cpu-hours counter is reset at the beginning of each month, so you can use the free plan every month. Awesome!

#### Control panel

Whenever you need to check what Codespaces environments you have running or created, you can do it in the [Codespaces panel][codespace-panel] in the GitHub UI.

![panel](https://gitlab.com/rdodin/pics/-/wikis/uploads/73aa0c2154959d4b25f2b1c2dbd1380b/image.png){.img-shadow}

The panel allows you to see and interact with the available Codespaces environments, including starting, stopping, and deleting them. You can also check what repositories are associated with each environment which is useful for a Containerlab user to quickly identify the lab environments.

/// details | Codespaces settings

Codespaces expose a bunch of per-user settings at the [github.com/settings/codespaces](https://github.com/settings/codespaces) page. The following settings are worth mentioning:

//// define

Secrets

- Secrets allow you to store sensitive information that will be available to your Codespaces environments. You can use them to store API keys, passwords, and other sensitive data that you don't want to expose in your code.

Setting Sync

- To make codespaces env feel like home, you can sync your settings across all your Codespaces environments. This includes themes, keybindings, and other settings that you have configured in your local VS Code instance.

Editor preference

- You can choose if you want to run the codespaces in a browser, in a local VS Code instance, or via a bridge to a JetBrains IDE.

////

///

#### Billing

It is always a good idea to periodically check how much you are spending with Codespaces against the free tier quota. Your billing information is available in the [GitHub settings][billing].

![billing](https://gitlab.com/rdodin/pics/-/wikis/uploads/4bf6eb89bd3b4f2d4b05ecdc3ab22675/image.png){.img-shadow}

The panel indicates that 10 cpu-hours out of 120 available were consumed in the current month' period and the codespaces environments occupy 8.15 GB of storage out of 15 GB included. So far it is all well within the free tier limits.

/// note
All users by default have a $0 spending limit[^4], which means that if you exceed the free tier limits, your environments will be stopped and you will **not** be charged. You can change this limit to a higher value if you want to be able to use Codespaces even after you exceed the free tier limits.
///

To avoid any surprises and lower your anxiety levels, GitHub Codespaces have two important settings that you configure at [github.com/settings/codespaces](https://github.com/settings/codespaces):

1. **[Idle timeout](https://docs.github.com/en/codespaces/setting-your-user-preferences/setting-your-timeout-period-for-github-codespaces)**  
    This setting allows you to "suspend" the running environment after a certain period of inactivity and defaults to 30 minutes. You can increase/decrease the timeout as you see fit. Consult with the docs to see what counts as activity and what doesn't.
2. **[Retention period](https://docs.github.com/en/codespaces/setting-your-user-preferences/configuring-automatic-deletion-of-your-codespaces)**  
    When you stopped the codespaces environment or it was suspended due to inactivity, it will be automatically deleted after a certain period of time. The default (and maximum) retention period is 30 days, but you can change it to a shorter period.  
    The stopped environment won't count against your cpu-hours quota, but it will still consume storage space, hence you might want to remove the stopped environments to free up the space.

## Onboarding a lab to Codespaces

## Dev Container

[devcontainers-doc]: https://containers.dev/
[codespaces-doc]: https://github.com/features/codespaces
[clab-devcontainer]: https://github.com/srl-labs/containerlab/pkgs/container/containerlab%2Fclab-devcontainer
[billing]: https://github.com/settings/billing/summary
[codespace-panel]: https://github.com/codespaces

[^1]: Check out the [Dev Container section](#dev-container) to learn more about the Containerlab' Dev Container package.
[^2]: At the moment of writing, GitHub Codespaces offer 120 cpu-hours/month and 15 GB storage for free to all users. See [here](#free-plan) for more details.
[^3]: A link points to the codespaces environment and refers a repo with the `.devcontainer` folder that defines the environment. For example: https://codespaces.new/srl-labs/srl-telemetry-lab?quickstart=1
[^4]: As indicated by the "Montly spending limit" text at the very bottom of the report table.
