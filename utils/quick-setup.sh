DISTRO_TYPE=""
SETUP_SSHD="${SETUP_SSHD:-true}"
CLAB_ADMINS="${CLAB_ADMINS:-true}"

# Docker version that will be installed by this install script.
DOCKER_VERSION="27.5.1"

function check_os {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        if [ "$ID" = "debian" ]; then
            DISTRO_TYPE="debian"
        elif [ "$ID" = "ubuntu" ]; then
            DISTRO_TYPE="ubuntu"
        elif [ "$ID" = "fedora" ]; then
            DISTRO_TYPE="fedora"
        elif [[ "$ID" = "rocky" || "$ID" = "rhel" || "$ID" = "centos" || "$ID" = "almalinux" ]]; then
            DISTRO_TYPE="rhel"
        else
            echo "This is not a supported OS. (Debian, Ubuntu, Fedora, Rocky, CentOS, RHEL, AlmaLinux)"
        fi
    else
        echo "Cannot determine the operating system"
    fi
}

function install-docker {
    # when this script is used to install just docker
    # we need to run check_os to detect the distro
    if [ -z "${DISTRO_TYPE}" ]; then
        check_os
    fi

    if [ "${DISTRO_TYPE}" = "debian" ]; then
        install-docker-debian
    elif [ "${DISTRO_TYPE}" = "ubuntu" ]; then
        install-docker-ubuntu
    elif [ "${DISTRO_TYPE}" = "rhel" ]; then
        install-docker-rhel
    elif [ "${DISTRO_TYPE}" = "fedora" ]; then
        install-docker-fedora
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

    DOCKER_PKG_NAME=$(apt-cache madison docker-ce | awk '{ print $3 }' | grep ${DOCKER_VERSION} | head -n 1)

    sudo apt-get -y install docker-ce=${DOCKER_PKG_NAME} docker-ce-cli=${DOCKER_PKG_NAME} containerd.io docker-buildx-plugin docker-compose-plugin
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

    DOCKER_PKG_NAME=$(apt-cache madison docker-ce | awk '{ print $3 }' | grep ${DOCKER_VERSION} | head -n 1)

    sudo apt-get -y install docker-ce=${DOCKER_PKG_NAME} docker-ce-cli=${DOCKER_PKG_NAME} containerd.io docker-buildx-plugin docker-compose-plugin
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

    DOCKER_PKG_NAME=$(yum list docker-ce --showduplicates | awk '{ print $2 }' | grep ${DOCKER_VERSION} | head -n 1)
    DOCKER_CLI_PKG_NAME=$(yum list docker-ce-cli --showduplicates | awk '{ print $2 }' | grep ${DOCKER_VERSION} | head -n 1)

    sudo yum install -y docker-ce-${DOCKER_PKG_NAME} docker-ce-cli-${DOCKER_CLI_PKG_NAME} containerd.io docker-buildx-plugin docker-compose-plugin

    # diverges from the instructions. This means docker daemon starts on each boot.
    sudo systemctl enable --now docker
}

function install-docker-fedora {
    # using instructions from:
    # https://docs.docker.com/engine/install/fedora/
    sudo dnf remove -y docker \
                  docker-client \
                  docker-client-latest \
                  docker-common \
                  docker-latest \
                  docker-latest-logrotate \
                  docker-logrotate \
                  docker-selinux \
                  docker-engine-selinux \
                  docker-engine

    sudo dnf install -y dnf-plugins-core

    if (( VERSION_ID >= 37 )); then
        sudo dnf-3 config-manager -y --add-repo https://download.docker.com/linux/fedora/docker-ce.repo
    else
        sudo dnf config-manager -y --add-repo https://download.docker.com/linux/fedora/docker-ce.repo
    fi

    if (( VERSION_ID >= 42 )); then
        # For compatability purposes
        DOCKER_VERSION="28.2.2"
    fi

    DOCKER_PKG_NAME=$(dnf list docker-ce --showduplicates | awk '{ print $2 }' | grep ${DOCKER_VERSION} | head -n 1)
    DOCKER_CLI_PKG_NAME=$(dnf list docker-ce-cli --showduplicates | awk '{ print $2 }' | grep ${DOCKER_VERSION} | head -n 1)

    sudo dnf install -y docker-ce-${DOCKER_PKG_NAME} docker-ce-cli-${DOCKER_CLI_PKG_NAME} containerd.io docker-buildx-plugin docker-compose-plugin

    # diverges from the instructions. This means docker daemon starts on each boot.
    sudo systemctl enable --now docker
}

function post-install-docker {
    # instructions from:
    # https://docs.docker.com/engine/install/linux-postinstall/
    sudo groupadd docker
    sudo usermod -aG docker "$SUDO_USER"
}

function setup-sshd {
    # increase max auth tries so unknown keys don't lock ssh attempts
    sudo sed -i 's/^#*MaxAuthTries.*/MaxAuthTries 50/' /etc/ssh/sshd_config

    if [[ "${DISTRO_TYPE}" = "rhel"  || "${DISTRO_TYPE}" = "fedora" ]]; then
        sudo systemctl restart sshd
    else
        sudo systemctl restart ssh
    fi
}

function install-make {
    if [[ "${DISTRO_TYPE}" = "rhel"  || "${DISTRO_TYPE}" = "fedora" ]]; then
        sudo dnf install -y make
    else
        sudo apt install -y make
    fi
}

function install-gh-cli {
    if [[ "${DISTRO_TYPE}" = "rhel"  || "${DISTRO_TYPE}" = "fedora" ]]; then
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

function setup-bash-prompt {
    # Check if the prompt is already set up
    if grep -q "function promptcmd" ~/.bashrc; then
        echo "Bash prompt already configured in .bashrc"
        return 1
    fi

    cat << 'EOF' >> ~/.bashrc

# Custom Bash Prompt Configuration
WHITE='\[\033[1;37m\]'; LIGHTRED='\[\033[1;31m\]'; LIGHTGREEN='\[\033[1;32m\]'; LIGHTBLUE='\[\033[1;34m\]'; DEFAULT='\[\033[0m\]'
cLINES=$WHITE; cBRACKETS=$WHITE; cERROR=$LIGHTRED; cSUCCESS=$LIGHTGREEN; cHST=$LIGHTGREEN; cPWD=$LIGHTBLUE; cCMD=$DEFAULT
promptcmd() { 
    PREVRET=$?
    PS1="\n"
    if [ $PREVRET -ne 0 ]; then 
        PS1="${PS1}${cBRACKETS}[${cERROR}x${cBRACKETS}]${cLINES}\342\224\200"
    else 
        PS1="${PS1}${cBRACKETS}[${cSUCCESS}*${cBRACKETS}]${cLINES}\342\224\200"
    fi
    PS1="${PS1}${cBRACKETS}[${cHST}\h${cBRACKETS}]${cLINES}\342\224\200"
    PS1="${PS1}[${cPWD}\w${cBRACKETS}]"
    PS1="${PS1}\n${cLINES}\342\224\224\342\224\200\342\224\200> ${cCMD}"
}
PROMPT_COMMAND=promptcmd
EOF

}

# keep SSH_AUTH_SOCK env var when using sudo
# to extract keys from the original user' agent
function add-ssh-socket-env-for-sudo {
    echo 'Defaults env_keep += "SSH_AUTH_SOCK"' | sudo tee /etc/sudoers.d/ssh_auth_sock
}

function install-containerlab {
    # when this script is used to install just containerlab
    # we need to run check_os to detect the distro
    if [ -z "${DISTRO_TYPE}" ]; then
        check_os
    fi

    if [ "${DISTRO_TYPE}" = "rhel" ]; then
        sudo yum-config-manager -y --add-repo=https://netdevops.fury.site/yum/ && \
        echo "gpgcheck=0" | sudo tee -a /etc/yum.repos.d/netdevops.fury.site_yum_.repo

        sudo yum install -y containerlab

    elif [ "${DISTRO_TYPE}" = "fedora" ]; then
        # Fedora 41 onwards ships with dnf5 instead of dnf 4 (packaged just as 'dnf')
        # and requires a slightly different syntax.
        if rpm --quiet -q dnf; then
            sudo dnf config-manager -y --add-repo "https://netdevops.fury.site/yum/" && \
            echo "gpgcheck=0" | sudo tee -a /etc/yum.repos.d/netdevops.fury.site_yum_.repo
        else  # dnf5
            sudo dnf config-manager addrepo --set=baseurl="https://netdevops.fury.site/yum/" && \
            echo "gpgcheck=0" | sudo tee -a /etc/yum.repos.d/netdevops.fury.site_yum_.repo
        fi

        sudo dnf install -y containerlab

    else
        echo "deb [trusted=yes] https://netdevops.fury.site/apt/ /" | \
        sudo tee -a /etc/apt/sources.list.d/netdevops.list

        sudo apt update -y && sudo apt install containerlab -y
    fi
}

function post-install-clab {
    if [ $(getent group clab_admins) ]; then
        echo "clab_admins group exists"
    else
      echo "Creating clab_admins group..."
      groupadd -r clab_admins 
    fi
    sudo usermod -aG clab_admins "$SUDO_USER"
}

function all {
    # check OS to determine distro
    check_os

    if [ "${SETUP_SSHD}" = "true" ]; then
        setup-sshd
    fi

    install-docker
    post-install-docker

    install-make
    install-gh-cli
    add-ssh-socket-env-for-sudo

    install-containerlab

    if [ "${CLAB_ADMINS}" = "true" ]; then
        post-install-clab
    fi
}

"$@"
