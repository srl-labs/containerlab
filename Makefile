BIN_DIR = $(CURDIR)/bin
BINARY = $(BIN_DIR)/containerlab

DATE := $(shell date)
COMMIT_HASH := $(shell git rev-parse --short HEAD)

LDFLAGS := -s -w -X 'github.com/srl-labs/containerlab/cmd.Version=0.0.0' -X 'github.com/srl-labs/containerlab/cmd.commit=$(COMMIT_HASH)' -X 'github.com/srl-labs/containerlab/cmd.date=$(DATE)'

include .mk/lint.mk

all: build

build:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BINARY) -ldflags="$(LDFLAGS)" main.go
	sudo chown root:root $(BINARY)
	sudo chmod 4755 $(BINARY)

build-linux-arm64:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=arm64 go build -o $(BINARY) -ldflags="$(LDFLAGS)" main.go
	sudo chown root:root $(BINARY)
	sudo chmod 4755 $(BINARY)

build-linux-amd64:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) -ldflags="$(LDFLAGS)" main.go
	sudo chown root:root $(BINARY)
	sudo chmod 4755 $(BINARY)

build-with-cover:
	mkdir -p $(BIN_DIR)
	go build -cover -o $(BINARY) -ldflags="$(LDFLAGS)" main.go
	sudo chown root:root $(BINARY)
	sudo chmod 4755 $(BINARY)

build-debug:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 go build -o $(BINARY) -gcflags=all="-N -l" -race -cover main.go
	sudo chown root:root $(BINARY)
	sudo chmod 4755 $(BINARY)

build-dlv-debug:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) -gcflags=all="-N -l" main.go
	sudo chown root:root $(BINARY)
	sudo chmod 4755 $(BINARY)


build-with-podman:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BINARY) -ldflags="$(LDFLAGS)" -trimpath -tags "podman exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper exclude_graphdriver_overlay containers_image_openpgp" main.go
	chmod a+x $(BINARY)
	sudo chown root:root $(BINARY)
	sudo chmod 4755 $(BINARY)

build-with-podman-debug:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 go build -o $(BINARY) -gcflags=all="-N -l" -race -cover -trimpath -tags "podman exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper exclude_graphdriver_overlay containers_image_openpgp" main.go
	sudo chown root:root $(BINARY)
	sudo chmod 4755 $(BINARY)

convert-coverage:
	go tool covdata textfmt -i=/tmp/clab-tests/coverage -o coverage.out

test:
	rm -rf /tmp/clab-tests/coverage
	mkdir -p /tmp/clab-tests/coverage
	CGO_ENABLED=1 go test -cover -race ./... -v -covermode atomic -args -test.gocoverdir="/tmp/clab-tests/coverage"


ifndef runtime
override runtime = docker
endif
ifndef suite
override suite = .
endif
robot-test: build-with-podman-debug
	sudo chown root:root $(BINARY) && sudo chmod 4755 $(BINARY)
	CLAB_BIN=$(BINARY) $$PWD/tests/rf-run.sh $(runtime) $$PWD/tests/$(suite)

MOCKDIR = ./mocks
.PHONY: mocks-gen
mocks-gen: mocks-rm ## Generate mocks for all the defined interfaces.
	go install go.uber.org/mock/mockgen@latest
	mockgen -package=mocknodes -source=nodes/node.go -destination=$(MOCKDIR)/mocknodes/node.go
	mockgen -package=mocks -source=core/dependency_manager/dependency_manager.go -destination=$(MOCKDIR)/dependency_manager.go
	mockgen -package=mockruntime -source=runtime/runtime.go -destination=$(MOCKDIR)/mockruntime/runtime.go
	mockgen -package=mocknodes -source=nodes/default_node.go -destination=$(MOCKDIR)/mocknodes/default_node.go
	mockgen -package=mocks -source=core/exec.go -destination=$(MOCKDIR)/exec.go

.PHONY: mocks-rm
mocks-rm: ## remove generated mocks
	rm -rf $(MOCKDIR)/*

lint:
	golangci-lint run



.PHONY: docs
docs:
	uv run --group docs zensical build --clean

.PHONY: site
site:
	uv run --group docs zensical serve -a 0.0.0.0:8000

# serve the site locally using zensical
.PHONY: serve-docs-full
serve-docs-full:
	uv run --group docs zensical serve -a 0.0.0.0:8001


# serve the site locally using zensical
# zensical performs incremental rebuilds, so content and navigation update automatically.
.PHONY: serve-docs
serve-docs:
	uv run --group docs zensical serve -a 0.0.0.0:8001

.PHONY: htmltest
htmltest:
	uv run --group docs zensical build --clean
	docker run --rm -v $(CURDIR):/test wjdp/htmltest --conf ./site/htmltest-w-github.yml
	rm -rf ./site

# build containerlab bin and push it as an OCI artifact to ttl.sh and ghcr registries
# to obtain the pushed artifact use: docker run --rm -v $(pwd):/workspace ghcr.io/deislabs/oras:v1.1.0 pull ttl.sh/<image-name>
.PHONY: oci-push
oci-push: build-with-podman
	@echo
# push to ttl.sh
#	docker run --rm -v $(CURDIR)/bin:/workspace ghcr.io/oras-project/oras:v1.1.0 push ttl.sh/clab-$(COMMIT_HASH):1d ./containerlab
#	@echo "download with: docker run --rm -v \$$(pwd):/workspace ghcr.io/oras-project/oras:v1.1.0 pull ttl.sh/clab-$(COMMIT_HASH):1d"
# push to ghcr.io
	@echo ""
	docker run --rm -v $(CURDIR)/bin:/workspace -v $${HOME}/.docker/config.json:/root/.docker/config.json ghcr.io/oras-project/oras:v1.1.0 push ghcr.io/srl-labs/clab-oci:$(COMMIT_HASH) ./containerlab
	@echo "With the following pull command you get a containerlab binary at your working directory. To use this downloaded binary - ./containerlab deploy.... Make sure not forget to add ./ prefix in order to use the downloaded binary and not the globally installed containerlab!"
	@echo 'If https proxy is configured in your environment, pass the proxies via --env HTTPS_PROXY="<proxy-address>" flag of the docker run command.'
	@echo "download with: sudo docker run --rm -v \$$(pwd):/workspace ghcr.io/oras-project/oras:v1.1.0 pull ghcr.io/srl-labs/clab-oci:$(COMMIT_HASH)"

oci-arm-push: build-linux-arm64
	@echo
	@echo ""
	docker run --rm -v $(CURDIR)/bin:/workspace -v $${HOME}/.docker/config.json:/root/.docker/config.json ghcr.io/oras-project/oras:v1.1.0 push ghcr.io/srl-labs/clab-oci:$(COMMIT_HASH) ./containerlab
	@echo "With the following pull command you get a containerlab binary at your working directory. To use this downloaded binary - do `chmod +x ./containerlab` and then `./containerlab deploy`. Make sure not forget to add ./ prefix in order to use the downloaded binary and not the globally installed containerlab!"
	@echo 'If https proxy is configured in your environment, pass the proxies via --env HTTPS_PROXY="<proxy-address>" flag of the docker run command.'
	@echo "download with: sudo docker run --rm -v \$$(pwd):/workspace ghcr.io/oras-project/oras:v1.1.0 pull ghcr.io/srl-labs/clab-oci:$(COMMIT_HASH)"
