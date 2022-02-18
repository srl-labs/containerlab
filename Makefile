BIN_DIR = $$(pwd)/bin
BINARY = $$(pwd)/bin/containerlab
MKDOCS_VER = 8.2.1
# insiders version/tag https://github.com/srl-labs/mkdocs-material-insiders/pkgs/container/mkdocs-material-insiders
MKDOCS_INS_VER = 4.8.3


all: build

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) -ldflags="-s -w -X 'github.com/srl-labs/containerlab/cmd.version=0.0.0' -X 'github.com/srl-labs/containerlab/cmd.commit=$$(git rev-parse --short HEAD)' -X 'github.com/srl-labs/containerlab/cmd.date=$$(date)'" main.go

build-with-podman:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BINARY) -ldflags="-s -w -X 'github.com/srl-labs/containerlab/cmd.version=0.0.0' -X 'github.com/srl-labs/containerlab/cmd.commit=$$(git rev-parse --short HEAD)' -X 'github.com/srl-labs/containerlab/cmd.date=$$(date)'" -trimpath -tags "podman exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper exclude_graphdriver_overlay containers_image_openpgp" main.go

test:
	go test -race ./... -v

lint:
	golangci-lint run

clint:
	docker run -it --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v1.40.1 golangci-lint run --timeout 5m -v

.PHONY: docs
docs:
	docker run -v $$(pwd):/docs --entrypoint mkdocs squidfunk/mkdocs-material:$(MKDOCS_VER) build --clean --strict

.PHONY: serve
site:
	docker run -it --rm -p 8000:8000 -v $$(pwd):/docs squidfunk/mkdocs-material:$(MKDOCS_VER)

# serve the site locally using mkdocs-material insiders container
.PHONY: serve-insiders
serve-insiders:
	docker run -it --rm -p 8001:8000 -v $$(pwd):/docs ghcr.io/srl-labs/mkdocs-material-insiders:$(MKDOCS_INS_VER)

.PHONY: htmltest
htmltest:
	docker run --rm -v $$(pwd):/docs --entrypoint mkdocs squidfunk/mkdocs-material:$(MKDOCS_VER) build --clean --strict
	docker run --rm -v $$(pwd):/test wjdp/htmltest --conf ./site/htmltest-w-github.yml
	rm -rf ./site

# build containerlab bin and push it as an OCI artifact to ttl.sh registry
# to obtain the pushed artifact use: docker run --rm -v $(pwd):/workspace ghcr.io/deislabs/oras:v0.11.1 pull ttl.sh/<image-name>
.PHONY: ttl-push
ttl-push: build-with-podman
	docker run --rm -v $$(pwd)/bin:/workspace ghcr.io/deislabs/oras:v0.11.1 push ttl.sh/clab-$$(git rev-parse --short HEAD):1d --manifest-config /dev/null:application/vnd.acme.rocket.config ./containerlab
	@echo "download with: docker run --rm -v \$$(pwd):/workspace ghcr.io/deislabs/oras:v0.11.1 pull ttl.sh/clab-$$(git rev-parse --short HEAD):1d"