This is a Go based repository that contains a cli tool for deploying network os containers to simulate networking topologies. Please follow these guidelines when contributing:

## Code Standards

### Required Before Each Commit

- The code should be formatted with `make format` command.

- The repository uses golangci-lint for linting Go code. Make sure linting passes with `make lint` command when golangci is locally installed or `make clint` when using docker container.

- Testing is done with `make test` command.

## Repository Structure
- The CLI tool source code is located in the `cmd/` directory.
- The core library code is located in the `core/` directory.
- The documentation files are located in the `docs/` directory.
- The constants files are located in the `constants/` directory.
- The files responsible to define nodes of the topology are located in the `nodes/` directory.
- The files responsible to define links of the topology are located in the `links/` directory.
- The files responsible to container runtime operations are located in the `runtime/` directory.
- The JSON schema files are located in the `schema/` directory.

## Key Guidelines

1. Follow Go best practices and idiomatic patterns
2. Maintain existing code structure and organization
3. Use table-driven tests for functions with multiple scenarios
4. Use cmp.Diff when writing tests to show differences between expected and actual values.
5. Write unit tests for new functionality.
6. Document public APIs and complex logic. Suggest changes to the `docs/` folder when appropriate