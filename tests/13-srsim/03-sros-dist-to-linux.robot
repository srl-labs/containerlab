*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot
Resource            ../srsim.robot

Suite Teardown      Destroy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}


*** Variables ***
${lab-name}         sr03
${lab-file-name}    03-srsim-dist.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Deploy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}

Ensure sros is reachable over ssh
    Login via SSH with username and password
    ...    address=clab-${lab-name}-sros-a
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=10

Verify links in node l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ip link show eth1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Ensure l1 can ping sros over 1/1/1 interface
    Sleep    30s    give some time for linecards to come up
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ping -c2 -w3 10.0.0.2"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    100% packet loss

Do a gNOI cert get-certs
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm --mount type=bind,source=${CURDIR}/clab-${lab-name}/.tls/ca,target=/tls ghcr.io/karimra/gnoic:0.2.0 cert get-certs --username admin --password 'NokiaSros1!' --tls-ca /tls/ca.pem --address clab-${lab-name}-sros-a
    Log    ${output}
    Should Contain    ${output}    sros-a.${lab-name}.io
