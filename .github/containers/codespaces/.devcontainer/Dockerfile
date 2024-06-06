FROM public.ecr.aws/docker/library/debian:12-slim

ARG CLAB_VERSION

RUN echo "deb [trusted=yes] https://netdevops.fury.site/apt/ /" | \
    tee -a /etc/apt/sources.list.d/netdevops.list

RUN if [ -z "$CLAB_VERSION" ]; then \
    apt update && apt install -y --no-install-recommends containerlab; \
    else \
    apt update && apt install -y --no-install-recommends containerlab=${CLAB_VERSION}; \
    fi

