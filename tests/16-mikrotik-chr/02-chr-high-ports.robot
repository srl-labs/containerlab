*** Settings ***
Library             OperatingSystem
Library             SSHLibrary
Resource            ../common.robot
Resource            ../ssh.robot

Suite Setup         Require AMD64
Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         16-02-chr-high-ports
${lab-file-name}    02-chr-high-ports.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    IF    ${rc} != 0    Fatal Error    Failed to deploy ${lab-name} lab

Wait for r1 and r2 to be healthy
    Wait Until Keyword Succeeds    300s    5s    Node is healthy    r1
    Wait Until Keyword Succeeds    60s    5s    Node is healthy    r2

Ensure r2 has all 64 interfaces
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r2
    ...    username=admin
    ...    password=admin
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    /interface ethernet print count-only
    Log    ${output}
    Should Be Equal As Integers    ${output}    64
    SSHLibrary.Close Connection

Ensure ether64 exists and forwards traffic across the third PCI bridge
    High port forwards traffic

Ensure restarting r1 preserves the high port and its configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} restart -t ${CURDIR}/${lab-file-name} --node r1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Wait Until Keyword Succeeds    300s    5s    Node is healthy    r1
    High port forwards traffic


*** Keywords ***
Require AMD64
    ${arch} =    Run    uname -m
    Skip If    '${arch}' != 'x86_64' and '${arch}' != 'amd64'    ether64 needs the AMD64 PCI-bridge layout

High port forwards traffic
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r1
    ...    username=admin
    ...    password=admin
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    /interface ethernet print count-only
    Log    ${output}
    Should Be Equal As Integers    ${output}    64
    Wait Until Keyword Succeeds    30s    2s    High port ping succeeds
    SSHLibrary.Close Connection

High port ping succeeds
    ${output} =    SSHLibrary.Execute Command    /ping 10.0.1.2 interface=ether64 count=3
    Log    ${output}
    Should Contain    ${output}    received=3

Node is healthy
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} inspect -f '{{.State.Health.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Strings    ${output}    healthy

Cleanup
    FOR    ${node}    IN    r1    r2
        ${rc}    ${output} =    Run And Return Rc And Output
        ...    sudo -E ${runtime} logs clab-${lab-name}-${node}
        Log    ${output}
    END
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
