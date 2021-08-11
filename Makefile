BIN_DIR = $(shell pwd)/bin
BINARY = $(shell pwd)/bin/containerlab
MKDOCS_VER = 7.2.2

all: build

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) main.go 

test:
	go test -race ./... -v

lint:
	golangci-lint run

clint:
	docker run -it --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v1.40.1 golangci-lint run -v

.PHONY: docs
docs:
	docker run -v $$(pwd):/docs --entrypoint mkdocs squidfunk/mkdocs-material:7.1.8 build --clean --strict

.PHONY: site
site:
	docker run -it --rm -p 8000:8000 -v $$(pwd):/docs squidfunk/mkdocs-material:$(MKDOCS_VER)