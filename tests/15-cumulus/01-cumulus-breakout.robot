*** Settings ***
Library             OperatingSystem
Library             SSHLibrary
Resource            ../common.robot
Resource            ../ssh.robot

Suite Setup         Run Keyword    Cleanup
Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}             cvx-breakout
${topo}                 01-cumulus-breakout.clab.yml
${runtime}              docker
${leaf}                 clab-${lab-name}-leaf
${username}             cumulus
${password}             Clab123!
${boot-timeout}         15 minutes
${ping-timeout}         2 minutes
${retry-interval}       15 seconds


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait for Cumulus VX to boot
    Wait Until Keyword Succeeds    ${boot-timeout}    ${retry-interval}
    ...    Leaf Log Should Contain    Startup complete

Client reaches the leaf over breakout lanes and a base port
    Login via SSH with username and password
    ...    address=${leaf}
    ...    username=${username}
    ...    password=${password}
    ...    try_for=30
    Configure Leaf Interface    swp1s0    10.0.1.2/24
    Configure Leaf Interface    swp2s3    10.0.2.2/24
    Configure Leaf Interface    swp3    10.0.3.2/24
    Configure Leaf Interface    swp8s1    10.0.4.2/24
    FOR    ${address}    IN    10.0.1.2    10.0.2.2    10.0.3.2    10.0.4.2
        Wait Until Keyword Succeeds    ${ping-timeout}    ${retry-interval}
        ...    Client Can Ping    ${address}
    END
    [Teardown]    SSHLibrary.Close All Connections


*** Keywords ***
Leaf Log Should Contain
    [Arguments]    ${text}
    ${rc}    ${output} =    Run And Return Rc And Output    docker logs ${leaf} 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${text}

Configure Leaf Interface
    [Arguments]    ${intf}    ${address}
    ${output}    ${rc} =    SSHLibrary.Execute Command
    ...    echo '${password}' | sudo -S sh -c 'ip addr add ${address} dev ${intf} && ip link set ${intf} up'
    ...    return_rc=True
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Client Can Ping
    [Arguments]    ${address}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${topo} --label clab-node-name\=client --cmd "ping -c2 -w3 ${address}"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    100% packet loss

Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${topo} --cleanup
