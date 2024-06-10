FROM mcr.microsoft.com/devcontainers/base:bookworm

ARG CLAB_VERSION

RUN echo "deb [trusted=yes] https://netdevops.fury.site/apt/ /" | \
    tee -a /etc/apt/sources.list.d/netdevops.list

RUN apt update && apt install -y --no-install-recommends \
    direnv

RUN if [ -z "$CLAB_VERSION" ]; then \
    apt update && apt install -y --no-install-recommends containerlab; \
    else \
    apt update && apt install -y --no-install-recommends containerlab=${CLAB_VERSION}; \
    fi

# install gNMIc and gNOIc
RUN bash -c "$(curl -sL https://get-gnmic.openconfig.net)" && bash -c "$(curl -sL https://get-gnoic.kmrd.dev)"

# add empty docker config files to avoid clab warnings for root user
RUN mkdir -p /root/.docker && touch /root/.docker/config.json

# vscode user is created in the MS devcontainer image
USER vscode

# create ssh key for vscode user to enable passwordless ssh to devices
RUN ssh-keygen -t ecdsa -b 256 -N "" -f ~/.ssh/id_ecdsa

# install pyenv
RUN bash -c "$(curl https://pyenv.run)"

# add empty docker config files to avoid clab warnings for vscode user
RUN mkdir -p /home/vscode/.docker && touch /home/vscode/.docker/config.json

# setup zsh/oh-my-zsh
COPY ./zsh/.zshrc /home/vscode/.zshrc
COPY ./zsh/.p10k.zsh /home/vscode/.p10k.zsh
COPY ./zsh/install-zsh-plugins.sh /tmp/install-zsh-plugins.sh
RUN bash -c "/tmp/install-zsh-plugins.sh"

# RUN sh -c "$(wget -O- https://github.com/deluan/zsh-in-docker/releases/download/v1.2.0/zsh-in-docker.sh)" -- \
#     -p git \
#     -p ssh-agent \
#     -p https://github.com/zsh-users/zsh-autosuggestions \
#     -p https://github.com/zsh-users/zsh-completions
