*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown

*** Variables ***
${lab-name}             bridge-ns-reconcile
${topo}                 34-bridge-ns-reconcile.clab.yml
${initial-vars}         34-bridge-ns-reconcile.vars.initial.yml
${add-c3-vars}          34-bridge-ns-reconcile.vars.add-c3.yml
${remove-c2-vars}       34-bridge-ns-reconcile.vars.remove-c2.yml
${move-bridge-vars}     34-bridge-ns-reconcile.vars.move-bridge.yml
${drift-parent-vars}    34-bridge-ns-reconcile.vars.drift-parent.yml
${no-bridge-vars}       34-bridge-ns-reconcile.vars.no-bridge.yml
${runtime}              docker
${bridge-cidr-gw}       192.168.0.1
${c1-if-ip}             192.168.0.2
${c2-if-ip}             192.168.0.3
${c3-if-ip}             192.168.0.4

*** Test Cases ***
Deploy ${lab-name} lab with two clients behind in-container bridge
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    ERROR

    Bridge Exists In Container    bp1    br01
    Container Interface Attached To Bridge    bp1    br01    c1eth1
    Container Interface Attached To Bridge    bp1    br01    c2eth1
    Ping From Node    c1    ${c2-if-ip}
    Ping From Node    c2    ${c1-if-ip}

Reconcile adds a new client to the in-container bridge
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${add-c3-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added nodes
    Should Contain    ${output}    added links
    Should Not Contain    ${output}    c1:eth1
    Should Not Contain    ${output}    c2:eth1

    Container Interface Attached To Bridge    bp1    br01    c3eth1
    Ping From Node    c1    ${c3-if-ip}
    Ping From Node    c3    ${c1-if-ip}
    Ping From Node    c2    ${c3-if-ip}

Added client reconcile is idempotent
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${add-c3-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    added nodes
    Should Not Contain    ${output}    added links
    Should Not Contain    ${output}    recreated

Reconcile removes one client from the in-container bridge
    ${c1_before} =    Node Runtime Identity    c1
    ${c3_before} =    Node Runtime Identity    c3
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${remove-c2-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted nodes
    Should Contain    ${output}    deleted endpoints
    Should Not Contain    ${output}    c1:eth1
    Should Not Contain    ${output}    c3:eth1
    Should Not Contain    ${output}    recreated

    Endpoint Interface Does Not Exist    bp1    c2eth1
    Ping From Node    c1    ${c3-if-ip}
    Ping From Node    c3    ${c1-if-ip}
    ${c1_after} =    Node Runtime Identity    c1
    ${c3_after} =    Node Runtime Identity    c3
    Should Be Equal As Strings    ${c1_after}    ${c1_before}
    Should Be Equal As Strings    ${c3_after}    ${c3_before}

Removal reconcile is idempotent
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${remove-c2-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    added links
    Should Not Contain    ${output}    deleted endpoints

Reconcile moves the bridge to a different parent container
    ${c1_before} =    Node Runtime Identity    c1
    ${c3_before} =    Node Runtime Identity    c3
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${move-bridge-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    br01|bp2
    Should Not Contain    ${output}    recreated

    Bridge Exists In Container    bp2    br01
    Container Interface Attached To Bridge    bp2    br01    c1eth1
    Container Interface Attached To Bridge    bp2    br01    c3eth1
    Endpoint Interface Does Not Exist    bp1    c1eth1
    Endpoint Interface Does Not Exist    bp1    c3eth1
    Bridge Does Not Exist In Container    bp1    br01
    Ping From Node    c1    ${c3-if-ip}
    Ping From Node    c3    ${c1-if-ip}
    ${c1_after} =    Node Runtime Identity    c1
    ${c3_after} =    Node Runtime Identity    c3
    Should Be Equal As Strings    ${c1_after}    ${c1_before}
    Should Be Equal As Strings    ${c3_after}    ${c3_before}

Bridge move is idempotent
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${move-bridge-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    added nodes
    Should Not Contain    ${output}    added links
    Should Not Contain    ${output}    deleted endpoints

Reconcile parent env drift recreates parent and bridge
    ${bp2_before} =    Node Runtime Identity    bp2
    ${c1_before} =    Node Runtime Identity    c1
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${drift-parent-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    bp2 (config drift: Env)
    Should Contain    ${output}    br01|bp2

    Bridge Exists In Container    bp2    br01
    Container Interface Attached To Bridge    bp2    br01    c1eth1
    Container Interface Attached To Bridge    bp2    br01    c3eth1
    Ping From Node    c1    ${c3-if-ip}
    Ping From Node    c3    ${c1-if-ip}
    ${bp2_after} =    Node Runtime Identity    bp2
    Should Not Be Equal As Strings    ${bp2_after}    ${bp2_before}

Parent drift reconcile is idempotent
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${drift-parent-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    added links
    Should Not Contain    ${output}    recreated nodes

Interrupted move recovery after reverting the move
    # Injection: a migration interrupted between park and restore leaves the
    # bridge-side half parked, carrying the owner bridge's altname. Re-applying
    # with the move reverted (same vars as the running lab) plans no migration
    # of its own; apply must recover the parked port through the restore path
    # instead of failing or silently stranding the link.
    ${park-ns} =    Set Variable    clab-park-clab-${lab-name}-br01|bp2
    ${bp2pid} =    Run    docker inspect -f '{{.State.Pid}}' clab-${lab-name}-bp2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip netns add '${park-ns}'
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo nsenter -t ${bp2pid} -n ip link set dev c1eth1 netns '${park-ns}'
    Should Be Equal As Integers    ${rc}    0
    Run    sudo ip netns exec '${park-ns}' ip link show dev c1eth1

    ${c1_before} =    Node Runtime Identity    c1
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${drift-parent-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    recovery of parked bridge port

    Container Interface Attached To Bridge    bp2    br01    c1eth1
    Container Interface Attached To Bridge    bp2    br01    c3eth1
    Ping From Node    c1    ${c3-if-ip}
    Ping From Node    c3    ${c1-if-ip}
    ${c1_after} =    Node Runtime Identity    c1
    Should Be Equal As Strings    ${c1_after}    ${c1_before}

Reconcile removes the in-container bridge from the topology
    # Removing the bridge node while its parent container keeps running must
    # also drop the bridge device that lives inside the parent, together with
    # its ports; the client-side veth halves die with their own pairs.
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${no-bridge-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted endpoints
    Should Not Contain    ${output}    recreated

    Bridge Does Not Exist In Container    bp2    br01
    Endpoint Interface Does Not Exist    bp2    c1eth1
    Endpoint Interface Does Not Exist    bp2    c3eth1

Bridge removal reconcile is idempotent
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${no-bridge-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    deleted endpoints
    Should Not Contain    ${output}    added links

*** Keywords ***
Teardown
    Run    ${CLAB_BIN} --runtime ${runtime} destroy --name ${lab-name} --cleanup -y

Setup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy --name ${lab-name} --cleanup -y 2>&1
    Log    ${output}

Run Clab Command
    [Arguments]    ${args}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ${args} 2>&1
    Log    ${output}
    RETURN    ${rc}    ${output}

Bridge Exists In Container
    [Arguments]    ${container}    ${bridge}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-${container} ip link show dev ${bridge}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    UP,LOWER_UP

Bridge Does Not Exist In Container
    [Arguments]    ${container}    ${bridge}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-${container} ip link show dev ${bridge}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

Container Interface Attached To Bridge
    [Arguments]    ${container}    ${bridge}    ${iface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-${container} ip link show dev ${iface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    master ${bridge}
    Should Contain    ${output}    state UP

Endpoint Interface Does Not Exist
    [Arguments]    ${container}    ${iface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-${container} ip link show dev ${iface}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

Ping From Node
    [Arguments]    ${node}    ${ip}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-${node} ping -c2 -W2 ${ip}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Node Runtime Identity
    [Documentation]    Returns the container's PID and start time for a node,
    ...    used to prove that a reconcile did not recreate the container:
    ...    a recreation changes both values.
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.State.Pid}} {{.State.StartedAt}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}
