*** Settings ***
Library             OperatingSystem
Resource            ../common.robot
Resource            ../ssh.robot

Suite Setup         Setup
Suite Teardown      Teardown


*** Variables ***
${lab-name}                 srsim-apply
${topo}                     11-srsim-apply.clab.yml
${initial-vars}             11-srsim-apply.vars.initial.yml
${add-link-vars}            11-srsim-apply.vars.add-link.yml
${add-node-vars}            11-srsim-apply.vars.add-node.yml
${component-change-vars}    11-srsim-apply.vars.component-change.yml
${runtime}                  docker
${runtime-cli-exec-cmd}     docker exec
${recovery-timeout}         3 minutes
${retry-interval}           10 seconds


*** Test Cases ***
Apply initial component-based SR-SIM lab
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deployed lab
    Should Contain    ${output}    ${lab-name}
    Node Should Be Running    sros-1
    Node Should Be Running    sros-a
    Component Label Should Equal    sros-1    clab-root-node-name    sros
    Component Label Should Equal    sros-1    clab-root-node-longname    clab-${lab-name}-sros
    Component Label Should Equal    sros-a    clab-root-node-name    sros
    Component Label Should Equal    sros-a    clab-root-node-longname    clab-${lab-name}-sros
    Interface Should Exist    client    eth1
    Interface Should Exist    sros-1    e1-1-c23-4
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Client Succeeds
    ...    10.0.1.2
    SR-SIM SSH Should Be Reachable    sros

Dry-run reports SR-SIM link addition
    ${rc}    ${output} =    Apply Topology    ${add-link-vars}    --dry-run
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply plan
    Should Contain    ${output}    added links
    Should Contain    ${output}    client:eth2
    Should Contain    ${output}    sros:e1-1-c23-3
    Interface Should Not Exist    client    eth2
    Interface Should Not Exist    sros-1    e1-1-c23-3

Apply adds link to existing component-based SR-SIM node
    ${sros_1_before} =    Node Runtime Identity    sros-1
    ${sros_a_before} =    Node Runtime Identity    sros-a
    ${rc}    ${output} =    Apply Topology    ${add-link-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply summary
    Should Contain    ${output}    added links
    Interface Should Exist    client    eth2
    Interface Should Exist    sros-1    e1-1-c23-3
    ${sros_1_after} =    Node Runtime Identity    sros-1
    ${sros_a_after} =    Node Runtime Identity    sros-a
    Should Be Equal As Strings    ${sros_1_after}    ${sros_1_before}
    Should Be Equal As Strings    ${sros_a_after}    ${sros_a_before}
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Client Succeeds
    ...    10.0.1.2
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Client Succeeds
    ...    10.0.2.2

Apply deletes link from existing component-based SR-SIM node
    ${sros_1_before} =    Node Runtime Identity    sros-1
    ${sros_a_before} =    Node Runtime Identity    sros-a
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted endpoints
    Interface Should Exist    client    eth1
    Interface Should Exist    sros-1    e1-1-c23-4
    Interface Should Not Exist    client    eth2
    Interface Should Not Exist    sros-1    e1-1-c23-3
    ${sros_1_after} =    Node Runtime Identity    sros-1
    ${sros_a_after} =    Node Runtime Identity    sros-a
    Should Be Equal As Strings    ${sros_1_after}    ${sros_1_before}
    Should Be Equal As Strings    ${sros_a_after}    ${sros_a_before}
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Client Succeeds
    ...    10.0.1.2

Apply adds SR-SIM node and link
    ${rc}    ${output} =    Apply Topology    ${add-node-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added nodes
    Should Contain    ${output}    extra
    Node Should Be Running    extra
    Interface Should Exist    client    eth3
    Interface Should Exist    extra    e1-1-c1-1
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Client Succeeds
    ...    10.0.3.2
    SR-SIM SSH Should Be Reachable    extra

Apply deletes SR-SIM node and link
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted nodes
    Should Contain    ${output}    extra
    Node Should Not Exist    extra
    Interface Should Not Exist    client    eth3

Dry-run rejects SR-SIM component layout change
    ${rc}    ${output} =    Apply Topology    ${component-change-vars}    --dry-run
    Should Not Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    distributed component layout changed
    Should Contain    ${output}    deploy --reconfigure


*** Keywords ***
Setup
    Run Clab Command    destroy --name ${lab-name} --cleanup

Teardown
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
    ...    ${runtime} inspect -f '{{.State.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

Node Runtime Identity
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.State.Pid}} {{.State.StartedAt}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

Component Label Should Equal
    [Arguments]    ${node}    ${label}    ${want}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{index .Config.Labels "${label}"}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    ${want}

Interface Should Exist
    [Arguments]    ${node}    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ip link show ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${interface}

Interface Should Not Exist
    [Arguments]    ${node}    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ip link show ${interface}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

Ping From Client Succeeds
    [Arguments]    ${destination}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-client ping -c 2 -W 2 ${destination}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    2 packets transmitted, 2 (packets )?received, 0% packet loss

SR-SIM SSH Should Be Reachable
    [Arguments]    ${node}
    Login via SSH with username and password
    ...    address=clab-${lab-name}-${node}
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=20
    ...    conn_timeout=2
    SSHLibrary.Close All Connections
