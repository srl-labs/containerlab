*** Settings ***
Library           OperatingSystem
Suite Teardown    Run Keyword    sudo containerlab destroy -t ${CURDIR}/01-linux-nodes.clab.yml --cleanup

*** Variables ***
${lab-name}       2-linux-nodes

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab deploy -t ${CURDIR}/01-linux-nodes.clab.yml
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Inspect ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab inspect -n ${lab-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec clab-${lab-name}-l1 ip link show eth1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec clab-${lab-name}-l1 ip link show eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in node l2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec clab-${lab-name}-l2 ip link show eth1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec clab-${lab-name}-l2 ip link show eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Destroy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab destroy -t ${CURDIR}/01-linux-nodes.clab.yml --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
