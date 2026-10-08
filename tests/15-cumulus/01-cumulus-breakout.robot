*** Settings ***
Library             OperatingSystem
Resource            ../common.robot

Suite Setup         Run Keyword    Cleanup
Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         cvx-breakout
${topo}             01-cumulus-breakout.clab.yml
${runtime}          docker
${leaf}             clab-${lab-name}-leaf
${boot-timeout}     15 minutes
${ping-timeout}     2 minutes
${retry-interval}   15 seconds


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait for Cumulus VX to boot
    Wait Until Keyword Succeeds    ${boot-timeout}    ${retry-interval}
    ...    Leaf Should Be Healthy

Client reaches the leaf over breakout lanes and base ports
    FOR    ${address}    IN    10.0.1.2    10.0.2.2    10.0.3.2    10.0.4.2    10.0.5.2
        Wait Until Keyword Succeeds    ${ping-timeout}    ${retry-interval}
        ...    Client Can Ping    ${address}
    END


*** Keywords ***
Leaf Should Be Healthy
    # boxen images report health only after the run process finished applying
    # the startup config, so this guarantees the L3 config is in place.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect --format {{.State.Health.Status}} ${leaf}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal    ${output}    healthy

Client Can Ping
    [Arguments]    ${address}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${topo} --label clab-node-name\=client --cmd "ping -c2 -w3 ${address}"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    100% packet loss

Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${topo} --cleanup
