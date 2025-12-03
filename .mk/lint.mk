GOFUMPT_CMD := docker run --rm -it -e GOFUMPT_SPLIT_LONG_LINES=on -v $(CURDIR):/work ghcr.io/hellt/gofumpt:v0.7.0
GOFUMPT_FLAGS := -l -w .

GODOT_CMD := docker run --rm -it -v $(CURDIR):/work ghcr.io/hellt/godot:1.4.11
GODOT_FLAGS := -w .

GOLANGCI_CMD := docker run -it --rm -v $(CURDIR):/app -w /app golangci/golangci-lint:v2.2.1 golangci-lint
GOLANGCI_LINT_FLAGS := run --timeout 5m -v
GOLANGCI_FMT_FLAGS := fmt -v

# when running in a CI env we use locally installed bind
ifdef CI
	GOFUMPT_CMD := GOFUMPT_SPLIT_LONG_LINES=on gofumpt
	GODOT_CMD := godot
	GOLANGCI_CMD := golangci-lint
endif

# lint with containerized golangci-lint
clint:
	${GOLANGCI_CMD} ${GOLANGCI_LINT_FLAGS}

format: golangci-format gofumpt godot # apply Go formatters

golangci-format:
	${GOLANGCI_CMD} ${GOLANGCI_FMT_FLAGS}

gofumpt:
	${GOFUMPT_CMD} ${GOFUMPT_FLAGS}

godot:
	${GODOT_CMD} ${GODOT_FLAGS}

golangci: # linting with golang-ci lint container
	${GOLANGCI_CMD} ${GOLANGCI_FLAGS}
