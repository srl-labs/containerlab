*** Comments ***
This test suite verifies
- the operation of tools veth create command


*** Settings ***
Library           OperatingSystem
Library           String
Suite Teardown    Run    sudo containerlab --runtime ${runtime} destroy -t ${topo} --cleanup

*** Variables ***
${lab-name}       2-linux-nodes
${topo}           ${CURDIR}/01-linux-nodes.clab.yml

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Create new veth pair between nodes
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} tools veth create -a clab-${lab-name}-l1:eth3 -b clab-${lab-name}-l2:eth3
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check the new interface has been created
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip netns exec clab-${lab-name}-l1 ip l show dev eth3
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    eth3
