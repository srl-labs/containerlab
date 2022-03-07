*** Settings ***
Library           OperatingSystem
Library           SSHLibrary
Suite Teardown    Run Keyword    Cleanup
Resource          ../common.robot

*** Variables ***
${lab-name}       04-01-ixiacone
${lab-file-name}  04-ixiacone01-clab.yml
${node1-name}     n1
${n1-mgmt-ip}
${ifc1-name}     eth1
${ifc2-name}     eth2
${ixiacone-ns-ifc-name}  eth1

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Get node mgmt IP
    ${rc}    ${n1-mgmt-ip} =    Run And Return Rc And Output
    ...    sudo docker inspect clab-${lab-name}-${node1-name} -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}'
    Should Be Equal As Integers    ${rc}    0

Verify link eth1 in ixia-c-one node n1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=n1 --cmd "docker exec -t ixia-c-port-dp-${ifc1-name} ip link show ${ixiacone-ns-ifc-name}"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify link eth2 in ixia-c-one node n1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=n1 --cmd "docker exec -t ixia-c-port-dp-${ifc2-name} ip link show ${ixiacone-ns-ifc-name}"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

*** Keywords ***
Cleanup
    Run    sudo containerlab destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}
