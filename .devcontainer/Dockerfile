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

# Switch to the vscode user provided by the base image
USER vscode

# Install Go 1.22
RUN bash -c 'ARCH=$(uname -m | sed "s/x86_64/amd64/" | sed "s/aarch64/arm64/") && \
    curl -L https://go.dev/dl/go1.22.9.linux-${ARCH}.tar.gz | sudo tar -C /usr/local -xzf - && \
    /usr/local/go/bin/go install -v golang.org/x/tools/gopls@latest && \
    /usr/local/go/bin/go install -v github.com/go-delve/delve/cmd/dlv@latest && \
    /usr/local/go/bin/go install -v honnef.co/go/tools/cmd/staticcheck@latest && \
    /usr/local/go/bin/go install -v github.com/cweill/gotests/gotests@latest && \
    /usr/local/go/bin/go install -v github.com/fatih/gomodifytags@latest && \
    /usr/local/go/bin/go install -v github.com/josharian/impl@latest && \
    /usr/local/go/bin/go install -v github.com/haya14busa/goplay/cmd/goplay@latest'

# Copy dclab script used to run the local containerlab build after `make build`
COPY ./.devcontainer/dclab /usr/local/bin/dclab

# Create SSH key for vscode user to enable passwordless SSH to devices
RUN ssh-keygen -t ecdsa -b 256 -N "" -f ~/.ssh/id_ecdsa

# Install pyenv
RUN bash -c "$(curl https://pyenv.run)"

# Add empty docker config files to avoid clab warnings for vscode user
RUN mkdir -p /home/vscode/.docker && echo "{}" > /home/vscode/.docker/config.json

# Setup Zsh and plugins
COPY ./.devcontainer/zsh/.zshrc /home/vscode/.zshrc
COPY ./.devcontainer/zsh/.p10k.zsh /home/vscode/.p10k.zsh
COPY ./.devcontainer/zsh/install-zsh-plugins.sh /tmp/install-zsh-plugins.sh
RUN bash -c "/tmp/install-zsh-plugins.sh"

# Setup pyenv virtual environment for clab tests
COPY tests/requirements.txt /tmp/requirements.txt
ENV PYENV_ROOT="/home/vscode/.pyenv"
ENV PATH="$PYENV_ROOT/shims:$PYENV_ROOT/bin:$PATH"
RUN pyenv virtualenv system clab-rf \
    && pyenv global clab-rf \
    && pip install -r /tmp/requirements.txt