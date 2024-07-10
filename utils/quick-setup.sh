DISTRO_TYPE=""

function check_os {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        if [ "$ID" = "debian" ]; then
            DISTRO_TYPE="debian"
        elif [ "$ID" = "ubuntu" ]; then
            DISTRO_TYPE="ubuntu"
        elif [[ "$ID" = "rocky" || "$ID" = "rhel" || "$ID" = "centos"  || "$ID" = "fedora" ]]; then
            DISTRO_TYPE="rhel"
        else
            echo "This is not Debian or Ubuntu"
        fi
    else
        echo "Cannot determine the operating system"
    fi
}

function install-docker {

    if [ "${DISTRO_TYPE}" = "debian" ]; then
        install-docker-debian
    elif [ "${DISTRO_TYPE}" = "ubuntu" ]; then
        install-docker-ubuntu
    elif [ "${DISTRO_TYPE}" = "rhel" ]; then
        install-docker-rhel
    else
        echo "Cannot determine the operating system"
        exit 1
    fi
}

function install-docker-debian {
    # using instructions from:
    # https://docs.docker.com/engine/install/debian/#install-using-the-repository
    for pkg in docker.io docker-doc docker-compose podman-docker containerd runc; do sudo apt-get remove -y $pkg; done

    # Add Docker's official GPG key:
    sudo apt-get update -y
    sudo apt-get install -y ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc

    # Add the repository to Apt sources:
    echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
    sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update -y

    sudo apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
}

function install-docker-ubuntu {
    # using instructions from:
    # https://docs.docker.com/engine/install/debian/#install-using-the-repository
    for pkg in docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc; do sudo apt-get remove -y $pkg; done

    # Add Docker's official GPG key:
    sudo apt-get update -y
    sudo apt-get install -y ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc

    # Add the repository to Apt sources:
    echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
    sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update -y

    sudo apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
}

function install-docker-rhel {
    # using instructions from:
    # https://docs.docker.com/engine/install/rhel/#install-using-the-repository
    sudo yum remove -y docker \
                  docker-client \
                  docker-client-latest \
                  docker-common \
                  docker-latest \
                  docker-latest-logrotate \
                  docker-logrotate \
                  docker-engine \
                  podman \
                  runc

    sudo yum install -y yum-utils
    sudo yum-config-manager -y --add-repo https://download.docker.com/linux/rhel/docker-ce.repo

    sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # diverges from the instructions. This means docker daemon starts on each boot.
    sudo systemctl enable --now docker
}

function setup-sshd {
    # increase max auth tries so unknown keys don't lock ssh attempts
    sudo sed -i 's/^#*MaxAuthTries.*/MaxAuthTries 50/' /etc/ssh/sshd_config

    if [ "${DISTRO_TYPE}" = "rhel" ]; then
        sudo systemctl restart sshd
    else
        sudo systemctl restart ssh
    fi
}

function install-gh-cli {
    if [ "${DISTRO_TYPE}" = "rhel" ]; then
        install-gh-cli-rhel
    else
        install-gh-cli-debian
    fi
}

function install-gh-cli-debian {
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && sudo apt update \
    && sudo apt install -y gh
}

function install-gh-cli-rhel {
    sudo dnf install -y 'dnf-command(config-manager)'
    sudo dnf config-manager -y --add-repo https://cli.github.com/packages/rpm/gh-cli.repo
    sudo dnf install git -y
    sudo dnf install -y gh --repo gh-cli
}

function install-containerlab {
    sudo bash -c "$(curl -sL https://get.containerlab.dev)"
}

function all {
    # check OS to determine distro
    check_os

    setup-sshd
    install-docker
    install-gh-cli

    install-containerlab
}

"$@"
