*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         srsim-new-chassis
${lab-file-name}    14-srsim-new-chassis.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check ixr-e2sc chassis and card
    Check chassis and card    ixr-e2sc    ixr-e2sc    cpm-ixr-e2sc/imm16-sfp++4-sfp28

Check ixr-e3 chassis and card
    Check chassis and card    ixr-e3    ixr-e3    cpm-ixr-e3/imm8-sfp112+18-sfp28+2-qsfpdd

Check sar-10x chassis and card
    Check chassis and card    sar-10x    sar-10x    cpiom-sar-x

Ensure new chassis containers stay running
    Sleep    30s    Let the simulators get past initial startup
    Check container is running    ixr-e2sc
    Check container is running    ixr-e3
    Check container is running    sar-10x

*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup

Check chassis and card
    [Arguments]    ${node}    ${chassis}    ${card}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect clab-${lab-name}-${node} --format '{{range .Config.Env}}{{println .}}{{end}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    NOKIA_SROS_CHASSIS=${chassis}
    Should Contain    ${output}    NOKIA_SROS_CARD=${card}

Check container is running
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect clab-${lab-name}-${node} --format '{{.State.Running}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${state} =    Strip String    ${output}
    Should Be Equal    ${state}    true
