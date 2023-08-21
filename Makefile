BIN_DIR = $(CURDIR)/bin
BINARY = $(CURDIR)/bin/containerlab
MKDOCS_VER = 9.1.4
# insiders version/tag https://github.com/srl-labs/mkdocs-material-insiders/pkgs/container/mkdocs-material-insiders
# make sure to also change the mkdocs version in actions' cicd.yml and force-build.yml files
MKDOCS_INS_VER = 9.1.4-insiders-4.32.4-hellt

DATE := $(shell date)
COMMIT_HASH := $(shell git rev-parse --short HEAD)

LDFLAGS := -s -w -X 'github.com/srl-labs/containerlab/cmd.version=0.0.0' -X 'github.com/srl-labs/containerlab/cmd.commit=$(COMMIT_HASH)' -X 'github.com/srl-labs/containerlab/cmd.date=$(DATE)'

include .mk/lint.mk

all: build

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) -ldflags="$(LDFLAGS)" main.go

build-with-cover:
	mkdir -p $(BIN_DIR)
	go build -cover -o $(BINARY) -ldflags="$(LDFLAGS)" main.go

build-debug:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) -gcflags=all="-N -l" -race -cover main.go


build-with-podman:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BINARY) -ldflags="$(LDFLAGS)" -trimpath -tags "podman exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper exclude_graphdriver_overlay containers_image_openpgp" main.go

build-with-podman-debug:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 go build -o $(BINARY) -gcflags=all="-N -l" -race -cover -trimpath -tags "podman exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper exclude_graphdriver_overlay containers_image_openpgp" main.go

convert-coverage:
	go tool covdata textfmt -i=./tests/coverage -o coverage.out

test:
	rm -rf $$PWD/tests/coverage
	mkdir -p $$PWD/tests/coverage
	CGO_ENABLED=1 go test -cover -race ./... -v -covermode atomic -args -test.gocoverdir="$$PWD/tests/coverage"


MOCKDIR = ./mocks
.PHONY: mocks-gen
mocks-gen: mocks-rm ## Generate mocks for all the defined interfaces.
	go install github.com/golang/mock/mockgen@v1.6.0
	mockgen -package=mocknodes -source=nodes/node.go -destination=$(MOCKDIR)/mocknodes/node.go
	mockgen -package=mocks -source=clab/dependency_manager/dependency_manager.go -destination=$(MOCKDIR)/dependency_manager.go
	mockgen -package=mockruntime -source=runtime/runtime.go -destination=$(MOCKDIR)/mockruntime/runtime.go
	mockgen -package=mocknodes -source=nodes/default_node.go -destination=$(MOCKDIR)/mocknodes/default_node.go
	mockgen -package=mocks -source=clab/exec/exec.go -destination=$(MOCKDIR)/exec.go

.PHONY: mocks-rm
mocks-rm: ## remove generated mocks
	rm -rf $(MOCKDIR)/*

lint:
	golangci-lint run

clint:
	docker run -it --rm -v $(CURDIR):/app -w /app golangci/golangci-lint:v1.47.1 golangci-lint run --timeout 5m -v

.PHONY: docs
docs:
	docker run -v $(CURDIR):/docs squidfunk/mkdocs-material:$(MKDOCS_VER) build --clean --strict

.PHONY: site
site:
	docker run -it --rm -p 8000:8000 -v $(CURDIR):/docs squidfunk/mkdocs-material:$(MKDOCS_VER)

# serve the site locally using mkdocs-material insiders container
.PHONY: serve-insiders
serve-docs:
	docker run -it --rm -p 8001:8000 -v $(CURDIR):/docs ghcr.io/srl-labs/mkdocs-material-insiders:$(MKDOCS_INS_VER)

# serve the site locally using mkdocs-material insiders container and dirty-reload
# in this mode navigation might not update properly, but the content will be updated
# if nav is not updated, re-run the target.
.PHONY: serve-docs
serve-insiders-dirty:
	docker run -it --rm -p 8001:8000 -v $(CURDIR):/docs ghcr.io/srl-labs/mkdocs-material-insiders:$(MKDOCS_INS_VER) serve -a 0.0.0.0:8000 --dirtyreload

.PHONY: htmltest
htmltest:
	docker run --rm -v $(CURDIR):/docs squidfunk/mkdocs-material:$(MKDOCS_VER) build --clean --strict
	docker run --rm -v $(CURDIR):/test wjdp/htmltest --conf ./site/htmltest-w-github.yml
	rm -rf ./site

# build containerlab bin and push it as an OCI artifact to ttl.sh and ghcr registries
# to obtain the pushed artifact use: docker run --rm -v $(pwd):/workspace ghcr.io/deislabs/oras:v0.11.1 pull ttl.sh/<image-name>
.PHONY: oci-push
oci-push: build-with-podman
	@echo
	@echo "With the following pull command you get a containerlab binary at your working directory. To use this downloaded binary - ./containerlab deploy.... Make sure not forget to add ./ prefix in order to use the downloaded binary and not the globally installed containerlab!"
	@echo 'If https proxy is configured in your environment, pass the proxies via --env HTTPS_PROXY="<proxy-address>" flag of the docker run command.'
# push to ttl.sh
#	docker run --rm -v $(CURDIR)/bin:/workspace ghcr.io/oras-project/oras:v0.12.0 push ttl.sh/clab-$(COMMIT_HASH):1d ./containerlab
#	@echo "download with: docker run --rm -v \$$(pwd):/workspace ghcr.io/oras-project/oras:v0.12.0 pull ttl.sh/clab-$(COMMIT_HASH):1d"
# push to ghcr.io
	@echo ""
	docker run --rm -v $(CURDIR)/bin:/workspace -v $${HOME}/.docker/config.json:/root/.docker/config.json ghcr.io/oras-project/oras:v0.12.0 push ghcr.io/srl-labs/clab-oci:$(COMMIT_HASH) ./containerlab
	@echo "download with: docker run --rm -v \$$(pwd):/workspace ghcr.io/oras-project/oras:v0.12.0 pull ghcr.io/srl-labs/clab-oci:$(COMMIT_HASH)"
