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
    ...    address=clab-${lab-name}-srsim10
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
    Wait Until Keyword Succeeds    2 minutes    10 seconds    Verify eth1 in node l1

Verify links in node l2
    Wait Until Keyword Succeeds    2 minutes    10 seconds    Verify eth1 in node l2

Check Cards after 40s on srsim10
    Sleep    40s    give some time for linecards to come up
    [Documentation]    Give some time for datapath cards to come up
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show card state | match ' up '" | sshpass -p 'NokiaSros1!' ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-srsim10
    Log    ${output}

Check Cards after 20s on srsim11
    Sleep    20s    give some time for linecards to come up
    [Documentation]    Give some time for datapath cards to come up
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show card state | match ' up '" | sshpass -p 'NokiaSros1!' ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-srsim11
    Log    ${output}

Ensure l1 can ping l2 via sr-sim network
    Wait Until Keyword Succeeds    2 minutes    10 seconds    From l1 ping l2 via sr-sim network

Check the number of hosts entries should be Equal to 4xIPv4 and 4xIPv6
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat /etc/hosts | grep -c clab-${lab-name}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Integers    ${output}    8

Do a gNMI GET using TLS
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm --mount type=bind,source=${CURDIR}/clab-${lab-name}/.tls/ca,target=/tls ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password 'NokiaSros1!' --tls-ca /tls/ca.pem --address clab-${lab-name}-srsim10 --path /state/system/oper-name --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    srsim10-a

Do a gNOI ping
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm --mount type=bind,source=${CURDIR}/clab-${lab-name}/.tls/ca,target=/tls ghcr.io/karimra/gnoic:0.1.0 system ping --username admin --password 'NokiaSros1!' --tls-ca /tls/ca.pem --address clab-${lab-name}-srsim10 --destination 10.78.140.3 --count 3
    Log    ${output}
    Should Contain    ${output}    3 packets sent
    Should Contain    ${output}    3 packets received

*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*

Verify eth1 in node l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ip link show eth1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify eth1 in node l2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l2 --cmd "ip link show eth1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

From l1 ping l2 via sr-sim network
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ping -c 2 -W 3 -M do -s 8662 10.111.0.1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss
    Should Contain    ${output}    2 received