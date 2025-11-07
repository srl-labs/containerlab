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

Create SSH keypair - ecdsa512
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ssh-keygen -t ecdsa -b 521 -N "" -f ${key-path}-ecdsa512

Ensure sros is reachable over ssh
    Login via SSH with username and password
    ...    address=clab-${lab-name}-srsim-classic
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=10

Ensure sros is reachable over ssh with public key ECDSA auth
    Login via SSH with public key
    ...    address=clab-${lab-name}-srsim-classic
    ...    username=admin
    ...    keyfile=${key-path}-ecdsa512
    ...    try_for=10

Check Classic Config Mode on srsim
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show system information | match Configuration | match Oper" | sshpass -p 'NokiaSros1!' ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-srsim-classic
    Log    ${output}
    Should Contain    ${output}    classic


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*
