*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         sr04
${lab-file-name}    04-srsim-classic.clab.yml
${runtime}          docker
${key-name}         clab-test-key
${gnmic_image}      ghcr.io/openconfig/gnmic:0.42.1
${gnmic_flags}      --username admin --password NokiaSros1! --values-only --skip-verify


*** Test Cases ***
Set key-path Variable
    ${key-path} =    OperatingSystem.Normalize Path    ~/.ssh/${key-name}
    Set Suite Variable    ${key-path}

Create SSH keypair - ecdsa512
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ssh-keygen -t ecdsa -b 521 -N "" -f ${key-path}-ecdsa512

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure sros is reachable over ssh
    Login via SSH with username and password
    ...    address=clab-${lab-name}-srsim
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=10

Ensure sros is reachable over ssh with public key ECDSA auth
    Login via SSH with public key
    ...    address=clab-${lab-name}-srsim
    ...    username=admin
    ...    keyfile=${key-path}-ecdsa512
    ...    try_for=10

Check Classic Config Mode on srsim
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-srsim get --path /state/system/management-interface/configuration-oper-mode
    Log    ${output}
    Should Contain    ${output}    classic



*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*
