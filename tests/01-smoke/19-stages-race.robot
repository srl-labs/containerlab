*** Settings ***
Documentation       This test ensures that when a lab consists of two nodes with a link between them,
...                 then even when one of the nodes starts after another, there is no effect on the exec
...                 command execution, since they should handle this gracefully.

Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-file}     stages-race.clab.yml
${lab-name}     stages-race
${runtime}      docker


*** Test Cases ***
Deploy ${lab-name} lab
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Not Contain    ${output.stderr}    ip: can't find device
    Should Not Contain    ${output.stderr}    Failed to execute command
    Should Not Contain    ${output.stderr}    ERRO

    Should Be Equal As Integers    ${output.rc}    0


*** Keywords ***
Teardown
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -c -t ${CURDIR}/${lab-file}

Setup
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -c -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'
