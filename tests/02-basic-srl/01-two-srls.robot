*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         02-01-two-srls
${lab-file-name}    02-srl02.clab.yml
${runtime}          docker
${key-name}         test


*** Test Cases ***
Create SSH keypair
    ${key-path} =    OperatingSystem.Normalize Path    ~/.ssh/${key-name}
    Log    ${key-path}
    Set Suite Variable    ${key-path}
    # Using ed25519 algo because of paramiko https://github.com/paramiko/paramiko/issues/1915
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ssh-keygen -t ed25519 -N "" -f ${key-path}

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node srl1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "ip link show e1-1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in node srl2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl2 --cmd "ip link show e1-1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify e1-1 interface have been admin enabled on srl1
    [Documentation]
    ...    This test cases ensures that e1-1 interface referenced in links section
    ...    has been automatically admin enabled
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "sr_cli 'show interface ethernet-1/1'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ethernet-1/1 is up

Verify srl2 accepted user-provided CLI config
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl2 --cmd "sr_cli 'info /system information location'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    test123

Verify saving config
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} save -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    ERRO

Ensure srl1 is reachable over ssh
    Login via SSH with username and password
    ...    address=clab-${lab-name}-srl1
    ...    username=admin
    ...    password=NokiaSrl1!
    ...    try_for=10

Ensure srl1 is reachable over ssh with public key auth
    Login via SSH with public key
    ...    address=clab-${lab-name}-srl1
    ...    username=admin
    ...    keyfile=${key-path}
    ...    try_for=10

Ensure srl1 can ping srl2 over ethernet-1/1 interface
    Sleep    5s    give some time for networking stack to settle
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "ip netns exec srbase-default ping 192.168.0.1 -c2 -w 3s"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Verify TLS works with JSON-RPC with skipping certificate check
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    curl -k 'https://admin:NokiaSrl1!@clab-${lab-name}-srl1/jsonrpc' -d '{"jsonrpc":"2.0","id":0,"method":"get","params":{"commands":[{"path":"/system/information/version","datastore":"state"}]}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    error

Verify TLS works with JSON-RPC and certificate check
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    curl --cacert ./clab-${lab-name}/.tls/ca/ca.pem 'https://admin:NokiaSrl1!@clab-${lab-name}-srl1/jsonrpc' -d '{"jsonrpc":"2.0","id":0,"method":"get","params":{"commands":[{"path":"/system/information/version","datastore":"state"}]}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    error

Verify TLS works with JSON-RPC, certificate check and IP address as SAN
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    curl --cacert ./clab-${lab-name}/.tls/ca/ca.pem 'https://admin:NokiaSrl1!@172.20.20.2/jsonrpc' -d '{"jsonrpc":"2.0","id":0,"method":"get","params":{"commands":[{"path":"/system/information/version","datastore":"state"}]}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    error


*** Keywords ***
Cleanup
    Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}
