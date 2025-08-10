*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         sr07
${lab-file-name}    07-srsim-mix.clab.yml
${runtime}          docker
${key-name}         clab-test-key


*** Test Cases ***
Set key-path Variable
    ${key-path} =    OperatingSystem.Normalize Path    ~/.ssh/${key-name}
    Set Suite Variable    ${key-path}

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure sros is reachable over ssh
    Login via SSH with username and password
    ...    address=clab-${lab-name}-srsim10-a
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=10

Ensure sros redirect port is open 
    ${rc}    ${output} =    Run And Return Rc And Output
    ...   sudo lsof -i :10022 | grep -c :10022
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Integers    ${output}    2

Verify links in node l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ip link show eth1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in node l2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l2 --cmd "ip link show eth1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Sleep for 10 seconds
    [Documentation]    Give some time for datapath cards to come up
    Sleep    10s

Ensure l1 can ping l2 via sr-sim network
    Sleep    30s    give some time for linecards to come up
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ping -c 2 -W 3 -M do -s 8662 10.111.0.1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss
    Should Contain    ${output}    2 received

Check the number of hosts entries should be Equal to 4xIPv4 and 4xIPv6
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat /etc/hosts | grep -c clab-${lab-name}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Integers    ${output}    8


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*
