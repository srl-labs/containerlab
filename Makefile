BIN_DIR = $(shell pwd)/bin
BINARY = $(shell pwd)/bin/containerlab

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
