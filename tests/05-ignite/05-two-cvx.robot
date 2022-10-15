*** Settings ***
Library           OperatingSystem
Library           SSHLibrary
Suite Teardown    Run Keyword    Cleanup
Resource          ../common.robot

*** Variables ***
${lab-name}       05-ignite
${lab-file-name}    05-cvx01-clab.yml
${node1-name}     sw1
${node2-name}     sw2
${sw1-mgmt-ip}
${sw2-mgmt-ip} 
${runtime}        docker

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure sw1 is reachable over ssh
    Common.Login via SSH with username and password
    ...    address=clab-${lab-name}-sw1
    ...    username=root
    ...    password=root
    ...    try_for=120

Ensure sw2 is reachable over ssh
    Common.Login via SSH with username and password
    ...    address=clab-${lab-name}-sw2
    ...    username=root
    ...    password=root
    ...    try_for=120

*** Keywords ***
Cleanup
    Run    sudo containerlab destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}
