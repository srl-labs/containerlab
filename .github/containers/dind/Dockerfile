FROM public.ecr.aws/docker/library/debian:12-slim

# Switch user to given USERNAME otherwise Ansible will be installed as root.
USER user
ENV PATH=$PATH:/home/${USERNAME}/.local/bin
