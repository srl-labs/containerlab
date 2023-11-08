FROM alpine:edge

RUN apk add openssh py3-twisted && \
    ssh-keygen -A  && \
    echo "done"

ADD docker-entrypoint.sh /usr/local/bin

CMD ["docker-entrypoint.sh"]
