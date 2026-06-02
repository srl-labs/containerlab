---
comments: true
hide:
- navigation
---

# Containerlab on Windows

By leveraging the [Windows Subsystem Linux (aka WSL)](https://learn.microsoft.com/en-us/windows/wsl/) you can run Containerlabs on Windows almost like on Linux. WSL allows Windows users to run a lightweight Linux VM inside Windows, and we can leverage this to run containerlab.

There are two primary ways of running Containerlab on Windows:

1. Running containerlab directly in the WSL VM.
2. Running containerlab inside a [Devcontainer](https://code.visualstudio.com/docs/devcontainers/containers) inside WSL.

We will cover both of these ways in this document, but first let's quickly go over the WSL setup.

## Setting up WSL

You need to have WSL installed on your system. So lets begin with getting WSL installed.

/// warning
Before starting, please ensure that hardware virtualization is supported on your system and **enabled** in your BIOS, The setting is likely called **SVM (AMD-V)** or **Intel VT-x** (depending on CPU and motherboard/system manufacturer).
///

### Windows 11

On Windows 11 in a Command Prompt or PowerShell window, you can simply execute:

```powershell
wsl --install
```

then reboot your system. WSL2 should now be installed.

### Windows 10

Newer versions of Windows 10 allow the same installation method as Windows 11. However for older versions you must follow a more manual process.

/// admonition | Note
    type: subtle-note
Instructions are from ['Manual installation steps for older versions of WSL'.](https://learn.microsoft.com/en-us/windows/wsl/install-manual)
///

Open an elevated PowerShell window (as administrator) and paste the following two commands:

```powershell
dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart

dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart
```

At this point restart your computer. After it has rebooted download the latest WSL2 kernel. [Download link](https://wslstorestorage.blob.core.windows.net/wslblob/wsl_update_x64.msi).

Follow the installation wizard. After completion finally set WSL2 as the default version of WSL.

In a Command Prompt or PowerShell window run the following:

```powershell
wsl --set-default-version 2
```

## WSL-Containerlab

While you can use a default WSL distributive, for a better user experience we recommend using the [WSL-Containerlab](https://github.com/srl-labs/WSL-Containerlab) distributive created by the Containerlab team.

Simply ensure you are using WSL 2.4.4 or newer. You can verify with the `wsl --version` command:

```powershell hl_lines="2"
wsl --version
WSL version: 2.4.11.0
Kernel version: 5.15.167.4-1
WSLg version: 1.0.65
MSRDC version: 1.2.5716
Direct3D version: 1.611.1-81528511
DXCore version: 10.0.26100.1-240331-1435.ge-release
Windows version: 10.0.19044.5247
```

### Installation

Download the .wsl file from the [releases page](https://github.com/srl-labs/wsl-containerlab/releases/latest).

Simply double click the file, and the distribution will install. You can also install with `wsl --install --from-file C:\path\to\clab.wsl`

You should be able to see "Containerlab" as a program in your start menu.

#### Initial setup

You can open Containerlab via the start menu, or in a Command Prompt/Powershell window, you can execute `wsl -d Containerlab`.

On the first startup of Containerlab WSL, there is an initial setup phase to help customise and configure the distro.

You will be presented with options to select the shell/prompt you want to use.

After that, SSH keys will be copied or generated (if no keys exist). This is to allow for paswordless SSH which enhances the [DevPod](#devpod) integration.

### Uninstallation

You can uninstall the distribution by executing `wsl --unregister Containerlab` in a Command Prompt or Powershell window.

## Manual WSL Setup

//// details | Manual installation steps
    type: subtle-question
WSL takes the central role in running containerlabs on Windows. Luckily, setting up WSL is very easy, and there are plenty of resources online from blogs to YT videos explaining the bits and pieces. Here we will provide some CLI-centric[^1] instructions that were executed on Windows 11.

/// admonition | Windows and WSL version
The following instructions were tested on Windows 11 and WSL2. On Windows 10 some commands may be different, but the general idea should be the same.

To check what WSL version you have, run `wsl -v` in your windows terminal.
///

First things first, open up a terminal and list the running WSL virtual machines and their versions:

```bash
wsl -l -v
```

<div class="embed-result">
```{.text .no-select .no-copy}
  NAME      STATE           VERSION
* Ubuntu    Running         2
```
</div>

On this system we already have a WSL VM with Ubuntu OS running, which was created when we installed WSL on Windows. If instead of a list of WSL VMs you get an error, you need to install WSL first:

```bash title="Installing WSL on Windows 11"
wsl --install -d Debian #(1)!
```

1. Installing a new WSL system will prompt you to choose a username and password.

If you performed a default WSL installation before, you are likely running an Ubuntu, and while it is perfectly fine to use it, we prefer Debian, so let's remove Ubuntu and install Debian instead:

```bash title="Removing Ubuntu and installing Debian"
wsl --unregister Ubuntu #(1)!
wsl --install -d Debian
```

1. Unregistering a WSL VM will remove the VM. You should reference a WSL instance by the name you saw in the `wsl -l -v` command.

Once the installation is complete, you will enter the WSL shell, which is a regular Linux shell[^2].

It is recommended to reboot your Windows system after installing the WSL. When the reboot is done you have a working WSL system that can run containerlab, congratulations!

**Installing docker and containerlab on WSL**

Now that we have a working WSL system, we can install docker and containerlab on it like we would on any Linux system.

/// danger | WSL2 and Docker Desktop
If you have Docker Desktop[^3] installed on your Windows system, you need to ensure that it is not enabled for the WSL VM that we intend to use for containerlabs.  
To check that your WSL system is "free" from Docker Desktop integration, run `sudo docker version` command and ensure that you have an error message saying that `docker` command is not found.

Check Docker Desktop settings to see how to disable Docker Desktop integration with WSL2 if the above command **does not** return an expected error.
///

We are going to use the [quick setup script](install.md#quick-setup) to install docker and containerlab, but since this script uses `curl`, we need to install it first:

```bash
sudo apt update && sudo apt -y install curl
```

and then run the quick setup script:

--8<-- "docs/install.md:quick-setup-script-cmd"

Now you should be able to run the `docker version` command and see the version of docker installed. That was easy, wasn't it?

The installation script also installs containerlab, so you can run `clab version` to see the version of containerlab installed. This means that containerlab is installed in the WSL VM and you can run containerlabs in a normal way, like you would on Linux.

////

## Devcontainer

Another convenient option to run containerlab on Windows (and [macOS](macos.md#devcontainer)) is to use the [Devcontainer](https://docs.github.com/en/codespaces/setting-up-your-project-for-codespaces/adding-a-dev-container-configuration/introduction-to-dev-containers) feature that works great with VS Code and many other IDE's.

A development container (or devcontainer) allows you to use a container as a full-featured development environment. By creating the `devcontainer.json`[^4] file, you define the development environment for your project. Containerlab project maintains a set of pre-built multi-arch devcontainer images that you can use to run containerlabs.  
It was initially created to power [containerlab in codespaces](manual/codespaces.md), but it is a perfect fit for running containerlab on a **wide range of OSes** such as macOS and Windows.

Since the devcontainer works exactly the same way on Windows and macOS, [please refer to the macOS](macos.md#devcontainer) section for the detailed documentation and a video walkthrough.

-{{youtube(url='https://www.youtube.com/embed/Xue1pLiO0qQ')}}-

A few things to keep in mind when using devcontainers on windows:

1. If using VS Code, you will need to install the server component in your WSL instance, this will require you to install `wget`, as VS Code installer requires it.

    ```bash
    sudo apt -y install wget
    ```

    Then you will be able to type `code .` in the cloned repository to open the project in VS Code.

2. As with macOS, you will likely wish to use a Docker-outside-of-Docker method, where the devcontainer will have access to the images and containers from the WSL VM.

## DevPod

DevPod delivers a stellar User Experience on macOS[^5], but on Windows, it requires a bit more setup. If using the [WSL distribution](https://github.com/srl-labs/WSL-Containerlab) this experience is easy as it gets on Windows.

Since SSH keys are copied in the [initial setup](#initial-setup). You simply have to configure the provider in DevPod.

![devpod-animation](https://gitlab.com/rdodin/pics/-/wikis/uploads/453ad6538927eded1bc5829b4b8fef40/devpod.gif)

And that's it! You should now be able to use DevPod to run containerlabs on Windows.

[^1]: If you don't have a decent terminal emulator on Windows, install "Windows Terminal" from the Microsoft Store.
[^2]: The kernel and distribution parameters can be checked as follows:

    ```bash
    roman@Win11:~$ uname -a
    Linux LAPTOP-H6R3238F 5.15.167.4-microsoft-standard-WSL2 #1 SMP Tue Nov 5 00:21:55 UTC 2024 x86_64 GNU/Linux
    ```

    ```bash
    roman@Win11:~$ cat /etc/os-release
    PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
    NAME="Debian GNU/Linux"
    VERSION_ID="12"
    ```

[^3]: Or any other desktop docker solution like Rancher Desktop, Podman Desktop, etc.
[^4]: Follows the devcontainer [specification](https://containers.dev/)
[^5]: Almost a [one-click solution](macos.md#devpod)
