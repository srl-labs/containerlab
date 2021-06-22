*** Comments ***
This test suite verifies
- connectivity of nodes to the linux bridge
- connectivity of nodes to the host netns

*** Settings ***
Library           OperatingSystem
Suite Setup       Run    sudo ip l del ${bridge-name}
Suite Teardown    Cleanup

*** Variables ***
${lab-name}       2-linux-nodes
${lab-file}       03-linux-nodes-to-bridge-and-host.clab.yml
${bridge-name}    br-01-03-clab
${br-link1-name}    l1-eth1
${br-link2-name}    l1-eth2
${host-link-name}    l1-01-03-eth3
${runtime}        docker

*** Test Cases ***
Create linux bridge
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link add name ${bridge-name} type bridge && sudo ip link set ${bridge-name} up
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in linux bridge
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${br-link1-name}
    Log    ${output}
    Should Contain    ${output}    master ${bridge-name} state UP
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${br-link2-name}
    Log    ${output}
    Should Contain    ${output}    master ${bridge-name} state UP

Verify links in host ns
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${host-link-name}
    Log    ${output}
    Should Contain    ${output}    state UP

*** Keywords ***
Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output    sudo containerlab --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}
    Run    sudo ip l del ${bridge-name}
    Run    sudo ip l del ${host-link-name}
