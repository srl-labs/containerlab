*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         1-sros
${lab-file-name}    1-sros.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait for SR OS to reach running state
    Sleep    1m
    Wait Until Keyword Succeeds    120    5s    SR OS is running


*** Keywords ***
SR OS is running
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-sros1 cat /healthy
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0 running

Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
