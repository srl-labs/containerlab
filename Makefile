BIN_DIR = $$(pwd)/bin
BINARY = $$(pwd)/bin/containerlab
MKDOCS_VER = 7.3.6

all: build

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) -ldflags="-s -w -X 'github.com/srl-labs/containerlab/cmd.version=0.0.0' -X 'github.com/srl-labs/containerlab/cmd.commit=$$(git rev-parse --short HEAD)' -X 'github.com/srl-labs/containerlab/cmd.date=$$(date)'" main.go

test:
	go test -race ./... -v

lint:
	golangci-lint run

clint:
	docker run -it --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v1.40.1 golangci-lint run --timeout 5m -v

.PHONY: docs
docs:
	docker run -v $$(pwd):/docs --entrypoint mkdocs squidfunk/mkdocs-material:$(MKDOCS_VER) build --clean --strict

.PHONY: site
site:
	docker run -it --rm -p 8000:8000 -v $$(pwd):/docs squidfunk/mkdocs-material:$(MKDOCS_VER)

.PHONY: htmltest
htmltest:
	docker run --rm -v $$(pwd):/docs --entrypoint mkdocs squidfunk/mkdocs-material:$(MKDOCS_VER) build --clean --strict
	docker run --rm -v $$(pwd):/test wjdp/htmltest --conf ./site/htmltest.yml
	rm -rf ./site