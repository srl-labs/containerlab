*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown


*** Variables ***
${lab-name}                 apply
${runtime}                  docker
${topo}                     29-apply.clab.yml
${initial-vars}             29-apply.vars.initial.yml
${add-link-vars}            29-apply.vars.add-link.yml
${add-special-links-vars}   29-apply.vars.add-special-links.yml
${add-node-vars}            29-apply.vars.add-node.yml
${runtime-cli-exec-cmd}     docker exec
${recovery-timeout}         30s
${retry-interval}           2s


*** Test Cases ***
Apply initial lab
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deployed lab
    Should Contain    ${output}    apply
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Exist    n1    eth1
    Interface Should Exist    n2    eth1
    Interface Should Exist    srlc    eth1
    Interface Should Exist    srl1    e1-1
    Configure Interface Address    n1    eth1    172.30.0.1/24
    Configure Interface Address    n2    eth1    172.30.0.2/24
    Configure Interface Address    srlc    eth1    172.31.0.1/24
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Node Succeeds
    ...    l1
    ...    172.17.0.2
    Ping From Node Succeeds    n1    172.30.0.2
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Node Succeeds
    ...    srlc
    ...    172.31.0.2

Dry-run reports link additions
    ${rc}    ${output} =    Apply Topology    ${add-link-vars}    --dry-run
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply plan
    Should Contain    ${output}    added links
    Should Contain    ${output}    l1:eth2
    Should Contain    ${output}    n1:eth2
    Should Contain    ${output}    srl1:e1-2
    Interface Should Not Exist    l1    eth2
    Interface Should Not Exist    l2    eth2
    Interface Should Not Exist    n1    eth2
    Interface Should Not Exist    n2    eth2
    Interface Should Not Exist    srlc    eth2
    Interface Should Not Exist    srl1    e1-2

Apply adds link between existing nodes
    ${n1_before} =    Node Runtime Identity    n1
    ${n2_before} =    Node Runtime Identity    n2
    ${srlc_before} =    Node Runtime Identity    srlc
    ${srl1_before} =    Node Runtime Identity    srl1
    ${rc}    ${output} =    Apply Topology    ${add-link-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply summary
    Should Contain    ${output}    added links
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Exist    l1    eth2
    Interface Should Exist    l2    eth2
    Interface Should Exist    n1    eth2
    Interface Should Exist    n2    eth2
    Interface Should Exist    srlc    eth2
    Interface Should Exist    srl1    e1-2
    ${n1_after} =    Node Runtime Identity    n1
    ${n2_after} =    Node Runtime Identity    n2
    ${srlc_after} =    Node Runtime Identity    srlc
    ${srl1_after} =    Node Runtime Identity    srl1
    Should Be Equal As Strings    ${n1_after}    ${n1_before}
    Should Be Equal As Strings    ${n2_after}    ${n2_before}
    Should Be Equal As Strings    ${srlc_after}    ${srlc_before}
    Should Be Equal As Strings    ${srl1_after}    ${srl1_before}
    Ping From Node Succeeds    n1    172.30.0.2
    Ping From Node Succeeds    srlc    172.31.0.2
    Configure Interface Address    n1    eth2    172.30.1.1/24
    Configure Interface Address    n2    eth2    172.30.1.2/24
    Ping From Node Succeeds    n1    172.30.1.2
    Configure Interface Address    srlc    eth2    172.31.1.1/24
    Configure SR Linux Interface Address    srl1    2    172.31.1.2/24
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Node Succeeds
    ...    srlc
    ...    172.31.1.2

Apply deletes link between existing nodes
    ${n1_before} =    Node Runtime Identity    n1
    ${n2_before} =    Node Runtime Identity    n2
    ${srlc_before} =    Node Runtime Identity    srlc
    ${srl1_before} =    Node Runtime Identity    srl1
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted endpoints
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Not Exist    l1    eth2
    Interface Should Not Exist    l2    eth2
    Interface Should Exist    n1    eth1
    Interface Should Exist    n2    eth1
    Interface Should Not Exist    n1    eth2
    Interface Should Not Exist    n2    eth2
    Interface Should Exist    srlc    eth1
    Interface Should Exist    srl1    e1-1
    Interface Should Not Exist    srlc    eth2
    Interface Should Not Exist    srl1    e1-2
    ${n1_after} =    Node Runtime Identity    n1
    ${n2_after} =    Node Runtime Identity    n2
    ${srlc_after} =    Node Runtime Identity    srlc
    ${srl1_after} =    Node Runtime Identity    srl1
    Should Be Equal As Strings    ${n1_after}    ${n1_before}
    Should Be Equal As Strings    ${n2_after}    ${n2_before}
    Should Be Equal As Strings    ${srlc_after}    ${srlc_before}
    Should Be Equal As Strings    ${srl1_after}    ${srl1_before}
    Ping From Node Succeeds    n1    172.30.0.2
    Ping From Node Succeeds    srlc    172.31.0.2

Dry-run reports supported non-veth link additions
    ${rc}    ${output} =    Apply Topology    ${add-special-links-vars}    --dry-run
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply plan
    Should Contain    ${output}    added links
    Should Contain    ${output}    l1:host1
    Should Contain    ${output}    l1:mgmt1
    Should Contain    ${output}    l2:dummy1
    Interface Should Not Exist    l1    host1
    Interface Should Not Exist    l1    mgmt1
    Interface Should Not Exist    l2    dummy1
    Host Interface Should Not Exist    apphost1
    Host Interface Should Not Exist    appmgmt1

Apply adds supported non-veth links
    ${rc}    ${output} =    Apply Topology    ${add-special-links-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added links
    Interface Should Exist    l1    host1
    Interface Should Exist    l1    mgmt1
    Interface Should Exist    l2    dummy1
    Host Interface Should Exist    apphost1
    Host Interface Should Exist    appmgmt1

Apply deletes supported non-veth links
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted endpoints
    Interface Should Not Exist    l1    host1
    Interface Should Not Exist    l1    mgmt1
    Interface Should Not Exist    l2    dummy1
    Host Interface Should Not Exist    apphost1
    Host Interface Should Not Exist    appmgmt1

Apply adds node and link
    ${rc}    ${output} =    Apply Topology    ${add-node-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added nodes
    Should Contain    ${output}    l3
    Node Should Be Running    l3
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Exist    l1    eth3
    Interface Should Exist    l3    eth1
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Node Succeeds
    ...    l1
    ...    172.17.0.2
    Configure Interface Address    l1    eth3    172.17.1.1/24
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Node Succeeds
    ...    l1
    ...    172.17.1.2

Apply deletes node and link
    ${rc}    ${output} =    Apply Topology    ${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted nodes
    Should Contain    ${output}    l3
    Node Should Not Exist    l3
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Not Exist    l1    eth3


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

Configure Interface Address
    [Arguments]    ${node}    ${interface}    ${address}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ip link set dev ${interface} up
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ip addr add ${address} dev ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Configure SR Linux Interface Address
    [Arguments]    ${node}    ${port}    ${address}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} bash -lc "printf '%s\n' 'set / interface ethernet-1/${port} admin-state enable' 'set / interface ethernet-1/${port} subinterface 0 admin-state enable' 'set / interface ethernet-1/${port} subinterface 0 ipv4 admin-state enable' 'set / interface ethernet-1/${port} subinterface 0 ipv4 address ${address}' 'set / network-instance default interface ethernet-1/${port}.0' 'commit now' | /opt/srlinux/bin/sr_cli -ed"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Host Interface Should Exist
    [Arguments]    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${interface}

Host Interface Should Not Exist
    [Arguments]    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show ${interface}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

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

Ping From Node Succeeds
    [Arguments]    ${node}    ${destination}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ping -c 2 -W 2 ${destination}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    2 packets transmitted, 2 (packets )?received, 0% packet loss
