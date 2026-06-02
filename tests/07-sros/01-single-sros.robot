*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Setup         Run Keyword    Setup
Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         1-sros
${lab-file-name}    1-sros.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    IF    ${rc} != 0    Fatal Error    Failed to deploy ${lab-name} lab

Wait for SR OS to reach running state
    Sleep    1m
    Wait Until Keyword Succeeds    120    5s    SR OS is running


*** Keywords ***
SR OS is running
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} exec clab-${lab-name}-sros1 cat /health
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0 running

Setup
    Run
    ...    cd ${CURDIR} && sudo -E ${runtime} run --rm -v $(pwd):/workspace ghcr.io/oras-project/oras:v1.1.0 pull registry.srlinux.dev/pub/sros-lic:23
    ${output} =    Run    cd ${CURDIR} && ls -la ./
    Log    ${output}

Cleanup
    # dump logs from sros1
    Run    sudo -E ${runtime} logs clab-${lab-name}-sros1 &> /tmp/${lab-name}-sros1.log
    ${contents} =    OperatingSystem.Get File    /tmp/${lab-name}-sros1.log
    Log    ${contents}

    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
