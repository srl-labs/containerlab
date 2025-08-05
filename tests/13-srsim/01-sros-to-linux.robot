*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         sr01
${lab-file-name}    01-srsim.clab.yml
${runtime}          docker
${key-name}         clab-test-key


*** Test Cases ***
Set key-path Variable
    ${key-path} =    OperatingSystem.Normalize Path    ~/.ssh/${key-name}
    Set Suite Variable    ${key-path}

Create SSH keypair - RSA
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ssh-keygen -t rsa -N "" -f ${key-path}-rsa

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure sros is reachable over ssh
    Login via SSH with username and password
    ...    address=clab-${lab-name}-sros
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=10

Ensure sros is reachable over ssh with public key RSA auth
    Login via SSH with public key
    ...    address=clab-${lab-name}-sros
    ...    username=admin
    ...    keyfile=${key-path}-rsa
    ...    try_for=10

Verify links in node l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ip link show eth1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Ensure l1 can ping sros over 1/1/c1/1 interface
    Sleep    5s    give some time for networking stack to settle
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ping 10.0.0.2 -c2 -w 3"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Do gNMI SET to change system name
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.41.0 set --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sros --update-path /configure/system/name --update-value thisismynewname
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Redeploy ${lab-name} lab to check startup config persistency
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} redeploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Do a gNMI GET and see if config changes after redeploy are persistent
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.41.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sros --path /state/system/oper-name --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    thisismynewname


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*
