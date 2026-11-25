*** Settings ***
Library             OperatingSystem
Resource            ../common.robot

Suite Setup         Clab Command    deploy -t ${topology}
Suite Teardown      Clab Command    destroy -t ${topology} --cleanup


*** Variables ***
${lab-name}         bngblaster
${topology}         ${CURDIR}/01-bngblaster.clab.yml
${runtime}          docker
${runtime-cli}      sudo ${runtime}
${lab-dir}          ${CURDIR}/clab-${lab-name}
${socket}           /var/bngblaster/clab/run.sock


*** Test Cases ***
Startup configuration starts the test once all links are attached
    ${config} =    Get File    ${lab-dir}/pppoe/clab/config.json
    Should Contain    ${config}    "type": "pppoe"
    ${env} =    Node Command    pppoe    env
    Should Contain    ${env}    BNGBLASTERCTRL_AUTOSTART=clab
    Should Contain    ${env}    BNGBLASTERCTRL_WAIT_INTERFACES=2
    ${env} =    Node Command    isis1    env
    Should Contain    ${env}    BNGBLASTERCTRL_WAIT_INTERFACES=2

Nodes report healthy
    FOR    ${node}    IN    pppoe    isis1    isis2    ctrl
        Wait Until Keyword Succeeds    120s    2s    Node Is Healthy    ${node}
    END

PPPoE sessions are established and their traffic is verified
    Wait Until Keyword Succeeds    60s    2s    Session Counters Contain    "sessions-established": 10
    Wait Until Keyword Succeeds    60s    2s    Session Counters Contain    "session-traffic-flows-verified": 20
    Wait Until Keyword Succeeds    60s    2s    Session Counters Contain    "stream-traffic-flows-verified": 20

IS-IS adjacency between two nodes comes up
    Wait Until Keyword Succeeds    60s    2s    IS-IS Adjacency Is Up    isis1    0100.0000.0002
    Wait Until Keyword Succeeds    60s    2s    IS-IS Adjacency Is Up    isis2    0100.0000.0001

Alias without startup-config only starts the controller
    ${env} =    Node Command    ctrl    env
    Should Not Contain    ${env}    BNGBLASTERCTRL_AUTOSTART
    Should Not Contain    ${env}    BNGBLASTERCTRL_WAIT_INTERFACES
    Directory Should Not Exist    ${lab-dir}/ctrl/clab

IPv6 is disabled on data interfaces only
    ${data} =    Node Command    isis1    cat /proc/sys/net/ipv6/conf/eth1/disable_ipv6
    Should Be Equal    ${data}    1
    ${mgmt} =    Node Command    isis1    cat /proc/sys/net/ipv6/conf/eth0/disable_ipv6
    Should Be Equal    ${mgmt}    0

Destroy tears the test down gracefully and keeps the report
    Clab Command    destroy -t ${topology}
    # SIGINT means the links were still present when the node was stopped;
    # the controller uses SIGTERM at once if they are already gone.
    ${log} =    Get File    ${lab-dir}/pppoe/clab/run.log
    Should Contain    ${log}    Received signal Interrupt
    Should Contain    ${log}    ALL SESSIONS TERMINATED
    ${report} =    Get File    ${lab-dir}/pppoe/clab/run_report.json
    Should Contain    ${report}    "sessions-established": 10
    # The linux node removes its link to isis1 at once when it is deleted, as
    # a device under test does, so isis1 must be stopped before any node of
    # the lab is deleted.
    FOR    ${node}    IN    isis1    isis2
        ${log} =    Get File    ${lab-dir}/${node}/clab/run.log
        Should Contain    ${log}    Received signal Interrupt
        Should Match Regexp    ${log}    ISIS L1 adjacency DOWN .* \\(teardown\\)
        File Should Exist    ${lab-dir}/${node}/clab/run_report.json
    END


*** Keywords ***
Clab Command
    [Arguments]    ${args}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ${args} 2>&1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

Node Command
    [Arguments]    ${node}    ${command}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli} exec clab-${lab-name}-${node} ${command} 2>&1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

Node Is Healthy
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli} inspect -f '{{.State.Health.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal    ${output}    healthy

Session Counters Contain
    [Arguments]    ${expected}
    ${output} =    Node Command    pppoe    bngblaster-cli ${socket} session-counters
    Should Contain    ${output}    ${expected}

IS-IS Adjacency Is Up
    [Arguments]    ${node}    ${peer}
    ${output} =    Node Command    ${node}    bngblaster-cli ${socket} isis-adjacencies
    Should Contain    ${output}    "system-id": "${peer}"
    Should Contain    ${output}    "adjacency-state": "Up"
