*** Settings ***
Library           OperatingSystem
Library           SSHLibrary
Library           Collections
Suite Teardown    Run Keyword    Cleanup
Resource          ../common.robot

*** Variables ***
${lab-name}       04-01-ixiacone
${lab-file-name}    04-ixiacone01-clab.yml
${ixia-node-name}    ixia
${ifc1-name}      eth1
${ifc2-name}      eth2
${runtime}        docker

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify link eth1 in keysight_ixia-c-one node n1
    Sleep    time_in_seconds=10 # give time for the link to come up
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=${ixia-node-name} --cmd "docker exec -t ixia-c-port-dp-${ifc1-name} ip link show ${ifc1-name}"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify link eth2 in keysight_ixia-c-one node n1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=${ixia-node-name} --cmd "docker exec -t ixia-c-port-dp-${ifc2-name} ip link show ${ifc2-name}"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

*** Keywords ***
Cleanup
    Run    sudo containerlab --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}
