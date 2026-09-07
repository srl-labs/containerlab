*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown


*** Variables ***
${lab-name}                 apply-netmode
${runtime}                  docker
${topo}                     31-apply-network-mode.clab.yml
${initial-vars}             31-apply-network-mode.vars.initial.yml
${add-sidecar1-vars}        31-apply-network-mode.vars.add-sidecar1.yml
${drift-w1-vars}            31-apply-network-mode.vars.drift-w1.yml
${add-together-vars}        31-apply-network-mode.vars.add-together.yml
${remove-w2-vars}           31-apply-network-mode.vars.remove-w2.yml
${link-recreate-vars}       31-apply-network-mode.vars.link-recreate.yml
${link-restart-vars}        31-apply-network-mode.vars.link-restart.yml
${runtime-cli-exec-cmd}     docker exec


*** Test Cases ***
Apply initial lab with a single node
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Node Should Be Running    w1

Apply adds a network-mode sidecar to an already-running target
    ${rc}    ${output} =    Apply Topology    ${add-sidecar1-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added nodes
    Should Contain    ${output}    w1-sc
    Node Should Be Running    w1-sc
    Nodes Should Share Network Namespace    w1    w1-sc

Apply cascades sidecar recreation when its network-mode target is recreated
    ${w1_before} =    Node Runtime Identity    w1
    ${w1sc_before} =    Node Runtime Identity    w1-sc
    ${rc}    ${output} =    Apply Topology    ${drift-w1-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    recreated nodes
    Should Contain    ${output}    w1
    Should Contain    ${output}    w1-sc
    Should Contain    ${output}    network-mode target
    ${w1_after} =    Node Runtime Identity    w1
    ${w1sc_after} =    Node Runtime Identity    w1-sc
    Should Not Be Equal As Strings    ${w1_after}    ${w1_before}
    Should Not Be Equal As Strings    ${w1sc_after}    ${w1sc_before}
    Nodes Should Share Network Namespace    w1    w1-sc

Apply adds a network-mode target and its sidecar together
    ${rc}    ${output} =    Apply Topology    ${add-together-vars}    --max-workers 1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added nodes
    Should Contain    ${output}    w2
    Should Contain    ${output}    w2-sc
    Node Should Be Running    w2
    Node Should Be Running    w2-sc
    Nodes Should Share Network Namespace    w2    w2-sc

Apply cascades sidecar recreation for a link change
    ${w1_before} =    Node Runtime Identity    w1
    ${sidecar_before} =    Node Runtime Identity    w1-sc
    ${rc}    ${output} =    Apply Topology    ${link-recreate-vars}    --dry-run
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    network-mode target
    ${rc}    ${output} =    Apply Topology    ${link-recreate-vars}
    Should Be Equal As Integers    ${rc}    0
    ${w1_after} =    Node Runtime Identity    w1
    ${sidecar_after} =    Node Runtime Identity    w1-sc
    Should Not Be Equal As Strings    ${w1_after}    ${w1_before}
    Should Not Be Equal As Strings    ${sidecar_after}    ${sidecar_before}
    Nodes Should Share Network Namespace    w1    w1-sc
    ${rc}    ${output} =    Apply Topology    ${add-together-vars}
    Should Be Equal As Integers    ${rc}    0

Apply restarts namespace dependents after a link-triggered target restart
    ${w1_before} =    Node Container ID    w1
    ${sidecar_before} =    Node Container ID    w1-sc
    ${rc}    ${output} =    Apply Topology    ${link-restart-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    network-mode target
    ${w1_after} =    Node Container ID    w1
    ${sidecar_after} =    Node Container ID    w1-sc
    Should Be Equal As Strings    ${w1_before}    ${w1_after}
    Should Be Equal As Strings    ${sidecar_before}    ${sidecar_after}
    Nodes Should Share Network Namespace    w1    w1-sc
    ${rc}    ${output} =    Apply Topology    ${add-together-vars}
    Should Be Equal As Integers    ${rc}    0

Apply rejects removing a network-mode target while its sidecar remains
    ${rc}    ${output} =    Apply Topology    ${remove-w2-vars}
    Should Not Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    w2-sc
    Should Contain    ${output}    w2
    Should Contain    ${output}    being removed from the topology
    Node Should Be Running    w2
    Node Should Be Running    w2-sc

Cleanup removes all incrementally added nodes
    Cleanup Lab


*** Keywords ***
Setup
    Cleanup Lab

Teardown
    Cleanup Lab

Cleanup Lab
    # Render every node, including nodes added with vars after the initial deployment.
    ${rc}    ${output} =    Run Clab Command
    ...    destroy -t ${CURDIR}/${topo} --vars ${CURDIR}/${add-together-vars} --cleanup
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${containers} =    Run And Return Rc And Output
    ...    ${runtime} ps -aq --filter label=containerlab=${lab-name}
    Should Be Equal As Integers    ${rc}    0
    Should Be Empty    ${containers}

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

Node Should Be Running
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.State.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    (?im)^running\\s*$

Node Runtime Identity
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.State.Pid}} {{.State.StartedAt}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

Node Container ID
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.Id}}' clab-${lab-name}-${node}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

Nodes Should Share Network Namespace
    [Arguments]    ${node_a}    ${node_b}
    ${rc}    ${netns_a} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node_a} readlink /proc/self/ns/net
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${netns_b} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node_b} readlink /proc/self/ns/net
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${netns_a}    ${netns_b}
