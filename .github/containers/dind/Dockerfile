FROM public.ecr.aws/docker/library/debian:12-slim

# Switch user to given USERNAME otherwise Ansible will be installed as root.
USER user
ENV PATH=$PATH:/home/${USERNAME}/.local/bin
# Install Ansible AVD collection.
RUN pip3 install "${ANSIBLE_CORE_VERSION}" \
    && ansible-galaxy collection install ${ANSIBLE_INSTALL_LOCATION} \
    && pip3 install -r /home/${USERNAME}/.ansible/collections/ansible_collections/arista/avd/requirements.txt \
    # install community.general to support callback plugins in ansible.cfg, etc.
    && ansible-galaxy collection install community.general