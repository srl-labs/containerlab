FROM alpine:3

LABEL maintainer="Roman Dodin <dodin.roman@gmail.com>"
LABEL documentation="https://containerlab.dev"
LABEL repo="https://github.com/srl-labs/containerlab"

RUN apk add --no-cache bash \
	curl \
	docker-cli \
	git \
	openssh \
	make \
	iptables \
	device-mapper \
	e2fsprogs-extra

COPY containerlab_*.apk /tmp/
RUN apk add --allow-untrusted /tmp/containerlab_*.apk && rm -f /tmp/containerlab_*.apk

CMD ["/usr/bin/containerlab", "help"]
