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



Check Cards after 20s on srsim11
    Sleep    20s    give some time for linecards to come up
    [Documentation]    Give some time for datapath cards to come up
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show card state | match ' up '" | sshpass -p 'NokiaSros1!' ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-srsim11
    Log    ${output}

    Wait Until Keyword Succeeds    2 minutes    10 seconds    Do a gNOI ping to other node

Do a gNMI GET using TLS
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm --mount type=bind,source=${CURDIR}/clab-${lab-name}/.tls/ca,target=/tls ghcr.io/openconfig/gnmic:0.42.1 get --username admin --password 'NokiaSros1!' --tls-ca /tls/ca.pem --address clab-${lab-name}-srsim10 --path /state/system/oper-name --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    srsim10-a

*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*


Do a gNOI ping to other node
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm --mount type=bind,source=${CURDIR}/clab-${lab-name}/.tls/ca,target=/tls ghcr.io/karimra/gnoic:0.2.0 system ping --username admin --password 'NokiaSros1!' --tls-ca /tls/ca.pem --address clab-${lab-name}-srsim10 --destination 100.0.0.11 --ns Base --count 3
    Log    ${output}
    Should Contain    ${output}    3 packets sent
    Should Contain    ${output}    3 packets received