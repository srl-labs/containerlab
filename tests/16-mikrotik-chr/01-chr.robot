*** Settings ***
Library             OperatingSystem
Library             SSHLibrary
Resource            ../common.robot
Resource            ../ssh.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         16-01-chr
${lab-file-name}    01-chr.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    IF    ${rc} != 0    Fatal Error    Failed to deploy ${lab-name} lab

Wait for r1 and r2 to be healthy
    Wait Until Keyword Succeeds    300s    5s    Node is healthy    r1
    Wait Until Keyword Succeeds    60s    5s    Node is healthy    r2

Ensure r1 is reachable over ssh with the default credentials and a lab default route
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r1
    ...    username=admin
    ...    password=admin
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    /system identity print
    Log    ${output}
    Should Contain    ${output}    name: r1
    ${output} =    SSHLibrary.Execute Command    /ping 10.0.0.2 count=3
    Log    ${output}
    Should Contain    ${output}    received=3
    SSHLibrary.Close Connection

Ensure r1 carries the containerlab management address on ether1
    ${rc}    ${mgmt-ip} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' clab-${lab-name}-r1
    Should Be Equal As Integers    ${rc}    0
    Login via SSH with username and password
    ...    address=${mgmt-ip}
    ...    username=admin
    ...    password=admin
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    /ip address print terse where interface=ether1
    Log    ${output}
    Should Contain    ${output}    address=${mgmt-ip}/
    SSHLibrary.Close Connection

Ensure r2 is reachable over ssh with custom credentials
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r2
    ...    username=netops
    ...    password=chr pass#1
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    /user print terse where name=admin
    Log    ${output}
    Should Contain    ${output}    X
    SSHLibrary.Close Connection

Ensure the rejected startup config of r3 is reported and r3 never becomes healthy
    Wait Until Keyword Succeeds    120s    5s    Node log contains    r3    startup config import FAILED
    ${output} =    Node log    r3
    Should Contain    ${output}    bad parameter network-type (line 2
    Should Not Contain    ${output}    RouterOS ready
    ${rc}    ${status} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} inspect -f '{{.State.Health.Status}}' clab-${lab-name}-r3
    Should Not Be Equal As Strings    ${status}    healthy

Save the configuration without the launcher's management objects
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} save -t ${CURDIR}/${lab-file-name} --node-filter r1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/r1/ftpboot/config.auto.rsc
    Log    ${f}
    Should Contain    ${f}    10.0.0.1/30
    Should Not Contain    ${f}    routing-table=mgmt
    Should Not Contain    ${f}    name=mgmt


*** Keywords ***
Node is healthy
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} inspect -f '{{.State.Health.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Strings    ${output}    healthy

Node log
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} logs clab-${lab-name}-${node}
    RETURN    ${output}

Node log contains
    [Arguments]    ${node}    ${text}
    ${output} =    Node log    ${node}
    Should Contain    ${output}    ${text}

Cleanup
    FOR    ${node}    IN    r1    r2    r3
        ${output} =    Node log    ${node}
        Log    ${output}
    END
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
