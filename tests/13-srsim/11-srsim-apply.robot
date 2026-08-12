*** Settings ***
Library             OperatingSystem
Resource            ../common.robot
Resource            ../ssh.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}                 srsim-apply
${topo}                     11-srsim-apply.clab.yml
${no-links-vars}            11-srsim-apply.vars.no-links.yml
${linked-vars}              11-srsim-apply.vars.linked.yml
${linked-with-node-vars}    11-srsim-apply.vars.linked-with-node.yml
${runtime}                  docker
${recovery-timeout}         3 minutes
${boot-timeout}             5 minutes
${retry-interval}           10 seconds


*** Test Cases ***
Apply initial lab without links has no client connectivity
    ${rc}    ${output} =    Apply    ${no-links-vars}
    Should Be Equal As Integers    ${rc}    0
    # a fresh deployment prints the container inspect table and no reconciliation summary
    Should Contain    ${output}    clab-${lab-name}-client
    Should Not Contain    ${output}    Apply summary
    # no links yet: client cannot reach either SR-SIM node
    Client Cannot Ping    10.0.1.2
    Client Cannot Ping    10.0.2.2

Apply adds links to SR-SIM nodes and connectivity succeeds
    ${rc}    ${output} =    Apply    ${linked-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added links
    Configure Client Interface    eth1    10.0.1.1/24
    Configure Client Interface    eth2    10.0.2.1/24
    # standalone SR-SIM
    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Client Can Ping    10.0.1.2
    # distributed SR-SIM
    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Client Can Ping    10.0.2.2

Stopping the SR-SIM nodes breaks connectivity
    Stop Node    R1
    Stop Node    R2
    Client Cannot Ping    10.0.1.2
    Client Cannot Ping    10.0.2.2

Apply restarts the stopped SR-SIM nodes and unparks their links
    ${rc}    ${output} =    Apply    ${linked-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    started nodes
    Should Contain    ${output}    Restored link
    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Client Can Ping    10.0.1.2
    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Client Can Ping    10.0.2.2

Apply adds a components-based SR-SIM node that boots with its cards up
    ${rc}    ${output} =    Apply    ${linked-with-node-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added nodes
    Should Contain    ${output}    R3
    # node boots and is reachable over SSH
    Wait Until Keyword Succeeds    ${boot-timeout}    ${retry-interval}    SR-SIM SSH Reachable    R3
    # both components (CPM slot A and line card slot 1) report up
    Wait Until Keyword Succeeds    ${boot-timeout}    ${retry-interval}    SR-SIM Cards Up    R3


*** Keywords ***
Apply
    [Arguments]    ${vars_file}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} apply -t ${CURDIR}/${topo} --vars ${CURDIR}/${vars_file} 2>&1
    Log    ${output}
    RETURN    ${rc}    ${output}

Stop Node
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} stop -t ${CURDIR}/${topo} -n ${node} 2>&1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Configure Client Interface
    [Arguments]    ${interface}    ${address}
    Run    ${runtime} exec clab-${lab-name}-client ip link set dev ${interface} up
    Run    ${runtime} exec clab-${lab-name}-client ip addr add ${address} dev ${interface}

Client Can Ping
    [Arguments]    ${destination}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-client ping -c2 -W2 ${destination}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    2 packets transmitted, 2 (packets )?received, 0% packet loss

Client Cannot Ping
    [Arguments]    ${destination}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-client ping -c2 -W2 ${destination}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

SR-SIM SSH Reachable
    [Arguments]    ${node}
    Login via SSH with username and password
    ...    address=clab-${lab-name}-${node}
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=20
    ...    conn_timeout=3
    SSHLibrary.Close All Connections

SR-SIM Cards Up
    [Arguments]    ${node}
    # 'show card state' for an sr-1-24d lists slot 1 (IOM), 1/1 (MDA) and A (CPM),
    # each with "<admin> <oper>" states. Assert both components are operationally up.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show card state" | sshpass -p 'NokiaSros1!' ssh -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null admin@clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # CPM (slot A): "A   cpm-1x   up   up"
    Should Match Regexp    ${output}    (?m)^A\\s+\\S+\\s+up\\s+up
    # IOM (slot 1): "1   i24-...   up   up"
    Should Match Regexp    ${output}    (?m)^1\\s+\\S+\\s+up\\s+up

Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${topo} --cleanup
