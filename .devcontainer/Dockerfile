FROM mcr.microsoft.com/devcontainers/python:3.11-bookworm

ARG CLAB_VERSION

# Add the netdevops repository
RUN echo "deb [trusted=yes] https://netdevops.fury.site/apt/ /" | \
    tee -a /etc/apt/sources.list.d/netdevops.list

# Install necessary packages, including curl
RUN apt-get update && apt-get install -y --no-install-recommends \
    direnv \
    btop \
    iputils-ping \
    tcpdump \
    iproute2 \
    qemu-kvm \
    dnsutils \
    telnet \
    curl

# Install Containerlab
RUN bash -c "$(curl -sL https://get.containerlab.dev)" -- -v ${CLAB_VERSION}

# Install GitHub CLI directly from the latest release
RUN bash -c 'ARCH=$(uname -m | sed "s/x86_64/amd64/" | sed "s/aarch64/arm64/") && \
    VERSION=$(curl -s https://api.github.com/repos/cli/cli/releases/latest | \
    grep -oP "\"tag_name\": \"\K[^\"]+") && \
    CLEAN_VERSION=${VERSION#v} && \
    DOWNLOAD_URL="https://github.com/cli/cli/releases/download/${VERSION}/gh_${CLEAN_VERSION}_linux_${ARCH}.tar.gz" && \
    curl -L "$DOWNLOAD_URL" | tar xz -C /tmp && \
    mv /tmp/gh_${CLEAN_VERSION}_linux_${ARCH}/bin/gh /usr/local/bin/ && \
    chmod +x /usr/local/bin/gh && \
    rm -rf /tmp/gh_${CLEAN_VERSION}_linux_${ARCH}'

# Install gNMIc and gNOIc
RUN bash -c "$(curl -sL https://get-gnmic.openconfig.net)" && \
    bash -c "$(curl -sL https://get-gnoic.kmrd.dev)"

# Add empty docker config files to avoid clab warnings for root user
RUN mkdir -p /root/.docker && echo "{}" > /root/.docker/config.json

# Maintain SSH_AUTH_SOCK env var when using sudo
RUN mkdir -p /etc/sudoers.d && echo 'Defaults env_keep += "SSH_AUTH_SOCK"' > /etc/sudoers.d/ssh_auth_sock

# Add vscode user to clab_admins group so that it can run sudo-less clab commands
# the group is created when clab is installed via the installation script
RUN usermod -aG clab_admins vscode

# Switch to the vscode user provided by the base image
USER vscode

# Copy dclab script used to run the local containerlab build after `make build`
COPY ./.devcontainer/dclab /usr/local/bin/dclab

# Create SSH key for vscode user to enable passwordless SSH to devices
RUN ssh-keygen -t ecdsa -b 256 -N "" -f ~/.ssh/id_ecdsa

# Install uv
COPY --from=ghcr.io/astral-sh/uv:0.6.2 /uv /uvx /bin/

# Add empty docker config files to avoid clab warnings for vscode user
RUN mkdir -p /home/vscode/.docker && echo "{}" > /home/vscode/.docker/config.json

# Setup Zsh and plugins
COPY ./.devcontainer/zsh/.zshrc /home/vscode/.zshrc
COPY ./.devcontainer/zsh/.p10k.zsh /home/vscode/.p10k.zsh
COPY ./.devcontainer/zsh/install-zsh-plugins.sh /tmp/install-zsh-plugins.sh
COPY ./.devcontainer/zsh/install-tools-completions.sh /tmp/install-tools-completions.sh
RUN bash -c "/tmp/install-zsh-plugins.sh && /tmp/install-tools-completions.sh"
