FROM mcr.microsoft.com/devcontainers/base:bookworm

ARG CLAB_VERSION

RUN echo "deb [trusted=yes] https://netdevops.fury.site/apt/ /" | \
    tee -a /etc/apt/sources.list.d/netdevops.list

RUN if [ -z "$CLAB_VERSION" ]; then \
    apt update && apt install -y --no-install-recommends containerlab; \
    else \
    apt update && apt install -y --no-install-recommends containerlab=${CLAB_VERSION}; \
    fi

# install gNMIc and gNOIc
RUN bash -c "$(curl -sL https://get-gnmic.openconfig.net)" && bash -c "$(curl -sL https://get-gnoic.kmrd.dev)"

# setup zsh/oh-my-zsh
COPY ./zsh/.zshrc /home/vscode/.zshrc
COPY ./zsh/.p10k.zsh /home/vscode/.p10k.zsh
COPY ./zsh/install-zsh-plugins.sh /tmp/install-zsh-plugins.sh
RUN bash -c "/tmp/install-zsh-plugins.sh"
