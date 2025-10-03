*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         sr05
${lab-file-name}    05-srsim-comp.clab.yml
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

Verify links in node l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ip link show eth1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Wait for linecards to come up
    Sleep    30s    give some time for linecards to come up

Ensure l1 can ping sros over 1/1/c1/1 interface
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "/bin/ping -c2 -w3

Ensure l1 can ping sros over 1/1/c23/4 interface
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "/bin/ping -c2 -w3 10.0.0.2"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Ensure MDA is overridden with explicit slot on sr1-01
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sr1-01 --path /state/card/mda/equipped-type --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    me12-100gb-qsfp28

Ensure MDA is overridden with implicit slot on sr1-02
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sr1-02 --path /state/card/mda/equipped-type --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    me12-100gb-qsfp28

Ensure MDA is overridden with env var on component on sr1-03
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sr1-03 --path /state/card/mda/equipped-type --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    me12-100gb-qsfp28

Ensure MDA is overridden with env var on node on sr1-04
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sr1-04 --path /state/card/mda/equipped-type --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    me12-100gb-qsfp28

Ensure XIOM is equipped and up on sr2s-01
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sr2s-01 --path /state/card/xiom/equipped-type --values-only
    Log    ${output}
    Should Contain    ${output}    iom-s-3.0t

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sr2s-01 --path /state/card/xiom/hardware-data/oper-state --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    in-service

Ensure XIOM MDA x/1 is equipped and up on sr2s-01
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sr2s-01 --path /state/card/xiom/mda/equipped-type --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ms18-100gb-qsfp28

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker run --network host --rm ghcr.io/openconfig/gnmic:0.42.0 get --username admin --password NokiaSros1! --insecure --address clab-${lab-name}-sr2s-01 --path /state/card/xiom/mda/hardware-data/oper-state --values-only
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    in-service


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*
