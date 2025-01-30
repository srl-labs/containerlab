*** Settings ***
Library     OperatingSystem
Resource    ../common.robot


*** Variables ***
${lab-name}     3-clab-gen
${runtime}      docker


*** Test Cases ***
Deploy ${lab-name} lab with generate command
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} generate --name ${lab-name} --kind linux --image ghcr.io/srl-labs/network-multitool --nodes 2,1,1 --deploy
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify nodes
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --name ${lab-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    clab-${lab-name}-node1-1
    Should Contain    ${output}    clab-${lab-name}-node1-2
    Should Contain    ${output}    clab-${lab-name}-node2-1
    Should Contain    ${output}    clab-${lab-name}-node3-1

    Cleanup    ${lab-name}

Deploy ${lab-name}-scale lab with generate command
    [Documentation]    Deploy 3-tier lab with 5 nodes in each tier. Tiers are interconnected with links.
    ...    This test verifies that scaled topology can be deployed without concurrent errors.
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} generate --name ${lab-name}-scale --kind linux --image alpine:3 --nodes 5,5,5 --deploy
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    failed
    Should Not Contain    ${output}    ERRO

    Cleanup    ${lab-name}-scale


*** Keywords ***
Cleanup
    [Arguments]    ${lab-name}

    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-name}.clab.yml --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    OperatingSystem.Remove File    ${lab-name}.clab.yml
