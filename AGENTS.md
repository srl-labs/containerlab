# Agents

Containerlab is a CLI tool for building and managing labs with containerized network devices. It provides a simple and efficient way to create both small and large network topologies using Docker containers.

## Repository structure

The repository is organized as follows:

- `.github/workflows`: Contains GitHub Actions workflows for CI/CD.
- `bin`: Directory where the built containerlab binary is placed.
- `cmd`: Contains the main command-line interface code for containerlab written with Cobra framework.
- `core`: Contains the core logic of containerlab, including lab management, topology parsing, and other functionalities.
- `docs`: Contains the documentation for containerlab, written with mkdocs-material framework.
- `nodes`: Contains the definitions and implementations of different types of network nodes that can be used in labs.
- `links`: Contains the logic for managing links between nodes in the lab.
- `tests`: Contains unit and integration tests for containerlab.
- `runtime`: Contains the runtime (docker and podman) logic for managing the lifecycle of labs and nodes.

## Documentation

Documentation is written with mkdocs-material framework and is present in the `docs` directory. To serve the documentation locally, use:

```bash
make serve-docs
```

To cleanup all started labs, use:

```bash
clab des -a -c -y
```

## Formatting

To format the code, use:

```bash
make format
```

## Building

The build and test automation is done in Makefile.

To build the clab binary, use:

```bash
make build
```

When building, the containerlab binary (alias `clab`) is placed in the `bin` directory.

## Testing

The unit tests are implemented with Go's built-in testing framework. To run the tests, use the following command:

```bash
make test
```

The integration tests are implemented with Robot Framework. To run the tests, use the following command:

```bash
CLAB_BIN=$(pwd)/bin/containerlab ./tests/rf-run.sh docker tests/<path to robot file>
```

When integration tests are run, the lab is destroyed automatically after the test ends.

To run lab deployment with debug logs, use `-d` flag.
