# Debugging

There is a point where you realized that putting another `fmt.Println("here100500")` is not enough. You need to have a debugger to inspect the state of your program.

## Debugging in VSCode

Debugging containerlab in VSCode relies on the [Go Dlv integration with VSCode](https://github.com/golang/vscode-go/wiki/debugging) and can be split into two categories:

1. Debugging using the root user
2. Debugging using the non-`root` user

Since containerlab requires the superuser privileges to run, the workflow will be slightly different depending on if operate as a `root` user already or not.

We will document the workflow for the latter (non-root user) case, as this is the most common scenario. In the non-root user case a developer should create a debug configuration file and optionally a task file to build the binary. The reason for the build step is rooted in a fact that we would need to build the binary first as our user, and then the debugger will be called as a `root` user and execute the binary with the debug mode.

### Create a debug configuration

The debug configuration defined in the `launch.json` file will contain the important fields such as `asRoot` and `console`, both needed for the debugging as a root user.

Here is an example of a configuration file that has debug configurations to run the following debug configurations:

1. run `containerlab tools vxlan create` command
2. run `containerlab deploy` command for a given topology
3. run `containerlab destroy` for a given topology

```{.json .code-scroll-lg}
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "tools vxlan create",
            "type": "go",
            "request": "launch",
            "mode": "exec",
            "console": "integratedTerminal",
            "asRoot": true,
            "program": "${workspaceFolder}/bin/containerlab",
            "args": [
                "tools",
                "vxlan",
                "create",
                "--remote",
                "10.0.0.20",
                "-l",
                "ens3"
            ],
            "preLaunchTask": "delete vx-ens3 interface",
        },
        {
            "name": "deploy linux lab",
            "type": "go",
            "request": "launch",
            "mode": "exec",
            "console": "integratedTerminal",
            "asRoot": true,
            "program": "${workspaceFolder}/bin/containerlab",
            "args": [
                "dep",
                "-t",
                "private/linux.clab.yml"
            ],
            "preLaunchTask": "delete linux lab",
        },
        {
            "name": "destroy linux lab",
            "type": "go",
            "request": "launch",
            "mode": "exec",
            "console": "integratedTerminal",
            "asRoot": true,
            "program": "${workspaceFolder}/bin/containerlab",
            "args": [
                "des",
                "-t",
                "private/linux.clab.yml"
            ],
            "preLaunchTask": "make build-dlv-debug",
        }
    ]
}
```

The debug config is run in the `exec` mode, which means that the debugger expects a program to be built first. This is why we need to create a task file to build the binary first.

The build happens via the `preLaunchTask` field, that references the task in a `tasks.json` file.

### Create a task file to build the binary

The [task file](https://code.visualstudio.com/docs/editor/tasks) provides the means to define arbitrary tasks that can be executed via VSCode UI and as well hooked up in the debug configuration.

Here is a simple task file that contains two tasks - one is building the binary with the debug information, and the other is a simple command that removes a test interface that the `tools vxlan create` command creates. The only task you need is the build task, but we wanted to show you how to define additional tasks that might be required before your run containerlab in the debug mode to cleanup from the previous execution.

The dependencies between the tasks are defined in the `dependsOn` field, and this is how you can first build the binary and then run the preparation step.

```{.json .code-scroll-lg}
{
    "version": "2.0.0",
    "tasks": [
        {
            "label": "delete vx-ens3 interface",
            "type": "shell",
            "command": "sudo ip link delete vx-ens3",
            "presentation": {
                "reveal": "always",
                "panel": "new"
            },
            "dependsOn": "make build-dlv-debug",
            "problemMatcher": []
        },
        {
            "label": "delete linux lab",
            "type": "shell",
            "command": "sudo clab des -c -t private/linux.clab.yml",
            "presentation": {
                "reveal": "always",
                "panel": "new"
            },
            "dependsOn": "make build-dlv-debug",
            "problemMatcher": []
        },
        {
            "label": "make build-dlv-debug",
            "type": "shell",
            "command": "make",
            "args": [
                "build-dlv-debug"
            ],
        }
    ]
}
```

Reach out via [Discord](https://discord.gg/vAyddtaEV9) to get help if you get stuck.
