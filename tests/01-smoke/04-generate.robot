*** Settings ***
Library           OperatingSystem
Suite Teardown    Cleanup

*** Variables ***
${lab-name}       3-clab-gen

*** Test Cases ***
Deploy ${lab-name} lab with generate command
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab generate --name ${lab-name} --kind linux --image alpine:latest --nodes 2,1,1 --deploy
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify nodes
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab inspect --name ${lab-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    clab-${lab-name}-node1-1
    Should Contain    ${output}    clab-${lab-name}-node1-2
    Should Contain    ${output}    clab-${lab-name}-node2-1
    Should Contain    ${output}    clab-${lab-name}-node3-1

*** Keywords ***
Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab destroy -t ${lab-name}.clab.yml --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    OperatingSystem.Remove File    ${lab-name}.clab.yml
