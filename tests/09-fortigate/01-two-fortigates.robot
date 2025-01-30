*** Settings ***
Library             OperatingSystem
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         forti
${lab-file-name}    fortigates.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    IF    ${rc} != 0    Fatal Error    Failed to deploy ${lab-name} lab

Wait for VM to reach running state
    Sleep    30s
    Wait Until Keyword Succeeds    120    5s    VM is healthy


*** Keywords ***
VM is healthy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} exec clab-${lab-name}-forti1 cat /health
    Log    ${output}

    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0 running

    ${rc2}    ${output2} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} exec clab-${lab-name}-forti2 cat /health
    Log    ${output2}

    Should Be Equal As Integers    ${rc2}    0
    Should Contain    ${output2}    0 running

Cleanup
    # dump logs from VM
    Run    sudo -E ${runtime} logs clab-${lab-name}-forti1 &> /tmp/${lab-name}-forti1.log
    ${contents} =    OperatingSystem.Get File    /tmp/${lab-name}-forti1.log
    Log    ${contents}

    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
