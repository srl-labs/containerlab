FROM mcr.microsoft.com/devcontainers/python:3.11-bookworm

ARG CLAB_VERSION

RUN echo "deb [trusted=yes] https://netdevops.fury.site/apt/ /" | \
    tee -a /etc/apt/sources.list.d/netdevops.list

# Create the update-github-cli-key.sh script
RUN echo '#!/bin/bash\n'\
'set -e\n'\
'echo "Updating GitHub CLI GPG key..."\n'\
'curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | \\\n'\
'sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg\n'\
'sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg\n' | \
sudo tee /usr/local/bin/update-github-cli-key.sh > /dev/null

# Make the script executable
RUN sudo chmod +x /usr/local/bin/update-github-cli-key.sh

# Create an apt configuration file to run the script before updates
RUN echo 'APT::Update::Pre-Invoke {"/usr/local/bin/update-github-cli-key.sh";};' | \
sudo tee /etc/apt/apt.conf.d/99updategithubclikey > /dev/null

# Fetch and install the GPG key for GitHub CLI (initial setup)
RUN /usr/local/bin/update-github-cli-key.sh

# Add the GitHub CLI repository
RUN echo "deb [arch=$(dpkg --print-architecture) \
    signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] \
    https://cli.github.com/packages stable main" | \
    sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null

# install containerlab and tools
RUN apt update && apt install -y --no-install-recommends containerlab${CLAB_VERSION:+=$CLAB_VERSION} \
    direnv \
    btop \
    gh \
    iputils-ping \
    tcpdump \
    iproute2 \
    qemu-kvm \
    dnsutils \
    telnet

# install gNMIc and gNOIc
RUN bash -c "$(curl -sL https://get-gnmic.openconfig.net)" && bash -c "$(curl -sL https://get-gnoic.kmrd.dev)"

# add empty docker config files to avoid clab warnings for root user
RUN mkdir -p /root/.docker && echo "{}" > /root/.docker/config.json

# maintain SSH_AUTH_SOCK env var when using sudo
RUN mkdir -p /etc/sudoers.d && echo 'Defaults env_keep += "SSH_AUTH_SOCK"' > /etc/sudoers.d/ssh_auth_sock

# vscode user is created in the MS devcontainer image
USER vscode

# copy dclab script that is used to run the local containerlab build
# after `make build` is executed
COPY ./.devcontainer/dclab /usr/local/bin/dclab

# create ssh key for vscode user to enable passwordless ssh to devices
RUN ssh-keygen -t ecdsa -b 256 -N "" -f ~/.ssh/id_ecdsa

# install pyenv
RUN bash -c "$(curl https://pyenv.run)"

# add empty docker config files to avoid clab warnings for vscode user
RUN mkdir -p /home/vscode/.docker && echo "{}" > /home/vscode/.docker/config.json

# setup zsh
COPY ./.devcontainer/zsh/.zshrc /home/vscode/.zshrc
COPY ./.devcontainer/zsh/.p10k.zsh /home/vscode/.p10k.zsh
COPY ./.devcontainer/zsh/install-zsh-plugins.sh /tmp/install-zsh-plugins.sh
RUN bash -c "/tmp/install-zsh-plugins.sh"

# setup pyenv venv for clab tests
COPY ./tests/requirements.txt /tmp/requirements.txt
ENV PYENV_ROOT /home/vscode/.pyenv
ENV PATH $PYENV_ROOT/shims:$PYENV_ROOT/bin:$PATH
RUN pyenv virtualenv system clab-rf && pyenv global clab-rf && pip install -r /tmp/requirements.txt
