FROM public.ecr.aws/docker/library/debian:12-slim

RUN echo "deb [trusted=yes] https://netdevops.fury.site/apt/ /" | \
    sudo tee -a /etc/apt/sources.list.d/netdevops.list

RUN apt update && apt install -y containerlab

# Switch to user
USER user
ENV PATH=$PATH:/home/${USERNAME}/.local/bin
