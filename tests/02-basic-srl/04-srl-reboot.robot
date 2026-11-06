*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         02-04-srl-reboot
${lab-file-name}    04-srl-reboot.clab.yml
${runtime}          docker
${srl1-container}   clab-${lab-name}-srl1


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify srl1 has the unless-stopped restart policy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect ${srl1-container} --format '{{.HostConfig.RestartPolicy.Name}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal    ${output}    unless-stopped

Ensure srl1 can ping srl2 after deploy
    Wait Until Keyword Succeeds    2 min    5 sec    srl1 can ping srl2

Reboot srl1 chassis
    ${restarts} =    Get srl1 restart count
    Set Suite Variable    ${restarts}
    # the exec session ends together with the rebooting node, so its result is not checked
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "sr_cli -d 'tools platform chassis reboot force'"
    Log    ${output}

Verify srl1 container was restarted by the runtime
    Wait Until Keyword Succeeds    2 min    5 sec    srl1 restart count is above    ${restarts}

Ensure srl1 can ping srl2 after chassis reboot
    Wait Until Keyword Succeeds    2 min    5 sec    srl1 can ping srl2

Ensure srl1 is reachable over ssh after chassis reboot
    Login via SSH with username and password
    ...    address=${srl1-container}
    ...    username=admin
    ...    password=NokiaSrl1!
    ...    try_for=10

Restart srl1 with containerlab
    [Documentation]
    ...    The ssh session opened by the previous test case is still active during the restart.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} restart -t ${CURDIR}/${lab-file-name} --node srl1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure srl1 can ping srl2 after containerlab restart
    Wait Until Keyword Succeeds    2 min    5 sec    srl1 can ping srl2

Destroy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify no lab containers are left after destroy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} ps -a --filter label\=containerlab\=${lab-name} --format '{{.Names}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Empty    ${output}


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup

srl1 can ping srl2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "ip netns exec srbase-default ping 192.168.0.1 -c2 -w 3"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    \\s0% packet loss

Get srl1 restart count
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect ${srl1-container} --format '{{.RestartCount}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

srl1 restart count is above
    [Arguments]    ${count}
    ${current} =    Get srl1 restart count
    Should Be True    ${current} > ${count}
