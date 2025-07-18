# Testing Containerlab

Containerlab's test program largely consists of:

- Go-based unit tests
- [RobotFramework](https://robotframework.org/)-based integration tests

## Integration Tests

The integration tests are written in RobotFramework and are located in the [`tests`][tests-dir] directory. The tests are run using the [`rf-run`][rf-run] command that wraps `robot` command. The tests are run in a Docker container, so you don't need to install RobotFramework on your local machine.

### Local execution

To execute the integration tests locally you have to install the python environment with the required dependencies. Containerlab uses [`uv`](https://docs.astral.sh/uv/) for all things Python, so getting the venv dialed in as as simple as:

```
uv sync
```

To make the Python venv setup with `uv` active in your current shell, you can source the following commands[^1]:

Usually you would run the tests using the locally built containerlab binary that contains the unreleased changes. The typical workflow then starts with building the containerlab binary:

```bash
export VIRTUAL_ENV=.venv
export PATH=$VIRTUAL_ENV/bin:~/sdk/go1.23.10/bin:$PATH #(1)!
```

1. `~/sdk/go1.23.10/bin` is the path to the matching Go SDK version [installed](https://go.dev/dl/).

To build the containerlab binary from the source code run:

```bash
make build
```

The newly built binary is located in the `bin` directory. In order to let the test runner script know where to find the binary, you have to set the `CLAB_BIN` environment variable before calling the `rf-run` script:

```bash
CLAB_BIN=$(pwd)/bin/containerlab ./tests/rf-run.sh <runtime> <test suite>
```

/// note
The test runner script requires you to specify the runtime as its first argument. The runtime can be either `docker` or `podman`. Containerlab primarily uses Docker as the default runtime, hence the number of tests written for docker outnumber the podman tests.
///

#### Selecting the test suite

Containerlab's integration tests are grouped by a topic, and each topic is mapped to a directory under the [`tests`][tests-dir] directory and RobotFramework allows for a flexible selection of tests/test suites to run. For example, to run all the smoke test cases, you can use the following command:

```bash
CLAB_BIN=$(pwd)/bin/containerlab ./tests/rf-run.sh docker tests/01-smoke
```

since [`01-smoke`][01-smoke-dir] is a directory containing all the smoke test suites.

Consequently, in order to run a specific test suite you just need to provide a path to it. E.g. running the `01-basic-flow.robot` test suite from the `01-smoke` directory:

```bash
CLAB_BIN=$(pwd)/bin/containerlab ./tests/rf-run.sh docker tests/01-smoke/01-basic-flow.robot
```

/// note
Selecting a specific test case in a test suite is not supported, since test suites are written in a way that test cases depend on previous ones.
///

#### Inspecting the test results

RobotFramework generates a detailed report in HTML and XML formats that can be found in the `tests/out` directory. The exact paths to the reports are printed to the console after the test run.

[tests-dir]: https://github.com/srl-labs/containerlab/tree/main/tests
[rf-run]: https://github.com/srl-labs/containerlab/blob/main/tests/rf-run.sh
[01-smoke-dir]: https://github.com/srl-labs/containerlab/tree/main/tests/01-smoke

[^1]: Tip: use direnv project to automatically set it when entering the directory.
