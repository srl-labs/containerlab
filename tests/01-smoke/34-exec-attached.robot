*** Comments ***
This suite tests the exec command with stdin and a TTY attached (--stdin and --tty) with the
local container runtimes.


*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup


*** Variables ***
${lab-name}         exec-attached
${topo}             ${CURDIR}/34-exec-attached.clab.yml
${runtime}          docker
${l1}               --label clab-node-name\=l1


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Exec passes stdin to the command
    ${result} =    Run Exec
    ...    printf 'line1\\nline2\\n' |
    ...    -t ${topo} ${l1} --stdin --cmd "sh -c 'cat > /tmp/stdin.txt'"
    Should Be Equal As Integers    ${result.rc}    0
    ${result} =    Run Exec    ${EMPTY}    -t ${topo} ${l1} -i --cmd "cat /tmp/stdin.txt"
    Should Be Equal As Integers    ${result.rc}    0
    Should Be Equal    ${result.stdout}    line1\nline2

Exec with stdin keeps stdout and stderr apart
    ${result} =    Run Exec
    ...    ${EMPTY}    -t ${topo} ${l1} -i --cmd "sh -c 'echo out; echo err >&2'"
    Should Be Equal As Integers    ${result.rc}    0
    Should Be Equal    ${result.stdout}    out
    Should Contain    ${result.stderr}    err

Exec with stdin exits with the exit code of the command
    ${result} =    Run Exec    ${EMPTY}    -t ${topo} ${l1} -i --cmd "sh -c 'exit 7'"
    Should Be Equal As Integers    ${result.rc}    7

Exec with stdin fails for a command that doesn't start
    Skip If    '${runtime}' == 'podman'    podman reports the exit code of a command that doesn't start
    ${result} =    Run Exec    ${EMPTY}    -t ${topo} ${l1} -i --cmd no-such-command
    Should Not Be Equal As Integers    ${result.rc}    0
    Should Be Empty    ${result.stdout}
    Should Contain    ${result.stderr}    failed to start

Exec with a TTY runs the command in a terminal
    ${result} =    Run Exec    ${EMPTY}    -t ${topo} ${l1} --tty --cmd tty
    Should Be Equal As Integers    ${result.rc}    0
    Should Start With    ${result.stdout}    /dev/pts/

Exec with stdin and a TTY needs a terminal
    ${result} =    Run Exec    ${EMPTY}    -t ${topo} ${l1} -i --tty --cmd sh
    Should Not Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stderr}    not a TTY

Exec with stdin and a TTY from a terminal exits with the exit code of the command
    ${result} =    Run Exec In A Terminal    -i --tty --cmd "sh -c 'exit 5'"
    Should Be Equal As Integers    ${result.rc}    5

Exec with stdin and a TTY starts the command with the size of the terminal
    ${result} =    Run Exec In A Terminal    -i --tty --cmd "stty size"    stty rows 30 cols 100;
    Should Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stdout}    30 100

Exec with stdin needs a filter matching one node
    ${result} =    Run Exec    ${EMPTY}    -t ${topo} -i --cmd true
    Should Not Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stderr}    matched 2


*** Keywords ***
Run Exec
    [Documentation]    Runs exec with the given args and stdin from the pipe, or from /dev/null
    ...    when the pipe is empty.
    [Arguments]    ${pipe}    ${args}
    IF    $pipe
        ${cmd} =    Set Variable    ${pipe} ${CLAB_BIN} --runtime ${runtime} exec ${args}
    ELSE
        ${cmd} =    Set Variable    ${CLAB_BIN} --runtime ${runtime} exec ${args} < /dev/null
    END
    ${result} =    Process.Run Process    ${cmd}    shell=True
    Log    stdout:${\n}${result.stdout}
    Log    stderr:${\n}${result.stderr}
    RETURN    ${result}

Run Exec In A Terminal
    [Documentation]    Runs exec on l1 with the given args in a terminal script creates, after
    ...    the setup command.
    [Arguments]    ${args}    ${setup}=${EMPTY}
    ${result} =    Process.Run Process
    ...    script    -qec    ${setup} ${CLAB_BIN} --runtime ${runtime} exec -t ${topo} ${l1} ${args}
    ...    /dev/null    stdin=/dev/null    timeout=60s
    Log    output:${\n}${result.stdout}
    RETURN    ${result}
