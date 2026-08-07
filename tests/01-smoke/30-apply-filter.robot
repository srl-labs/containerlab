*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Cleanup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}                 apply-filter
${runtime}                  docker
${topo}                     30-apply-filter.clab.yml
${initial-vars}             30-apply-filter.vars.initial.yml
${add-group-vars}           30-apply-filter.vars.add-group.yml
${runtime-cli-exec-cmd}     docker exec


*** Test Cases ***
Filtered apply adds shared-netns group without touching existing nodes
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    ${provider_before} =    Node Runtime Identity    provider
    ${unrelated1_before} =    Node Runtime Identity    unrelated1
    ${unrelated2_before} =    Node Runtime Identity    unrelated2

    ${rc}    ${output} =    Apply Topology
    ...    ${add-group-vars}
    ...    --node-filter new-child1,new-child2
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added nodes
    Should Contain    ${output}    new-provider
    Should Contain    ${output}    new-child1
    Should Contain    ${output}    new-child2
    Should Not Contain    ${output}    deleted endpoints

    Node Should Be Running    new-provider
    Node Should Be Running    new-child1
    Node Should Be Running    new-child2
    Interface Should Exist    provider    eth1
    Interface Should Exist    unrelated1    eth1
    Interface Should Exist    unrelated1    eth2
    Interface Should Exist    unrelated2    eth1
    Interface Should Exist    unrelated2    eth2
    Interface Should Exist    new-provider    eth1
    Interface Should Exist    new-child1    eth1
    Interface Should Exist    new-child2    eth1

    ${provider_after} =    Node Runtime Identity    provider
    ${unrelated1_after} =    Node Runtime Identity    unrelated1
    ${unrelated2_after} =    Node Runtime Identity    unrelated2
    Should Be Equal As Strings    ${provider_after}    ${provider_before}
    Should Be Equal As Strings    ${unrelated1_after}    ${unrelated1_before}
    Should Be Equal As Strings    ${unrelated2_after}    ${unrelated2_before}

    ${rc}    ${output} =    Run Clab Command    destroy --name ${lab-name} --cleanup
    Should Be Equal As Integers    ${rc}    0
    Node Should Not Exist    provider
    Node Should Not Exist    child1
    Node Should Not Exist    child2
    Node Should Not Exist    unrelated1
    Node Should Not Exist    unrelated2
    Node Should Not Exist    new-provider
    Node Should Not Exist    new-child1
    Node Should Not Exist    new-child2


*** Keywords ***
Cleanup
    Run Clab Command    destroy --name ${lab-name} --cleanup

Run Clab Command
    [Arguments]    ${args}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ${args} 2>&1
    Log    ${output}
    RETURN    ${rc}    ${output}

Apply Topology
    [Arguments]    ${vars_file}    ${extra_args}=${EMPTY}
    ${rc}    ${output} =    Run Clab Command
    ...    apply -t ${CURDIR}/${topo} --vars ${CURDIR}/${vars_file} ${extra_args}
    RETURN    ${rc}    ${output}

Interface Should Exist
    [Arguments]    ${node}    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ip link show ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${interface}

Node Should Be Running
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.State.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    (?im)^running\\s*$

Node Should Not Exist
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect clab-${lab-name}-${node}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

Node Runtime Identity
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.State.Pid}} {{.State.StartedAt}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}
