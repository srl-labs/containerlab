*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../ssh.robot
Resource            ../common.robot
Resource            ../srsim.robot

Suite Setup         Skip If    '${26_10_TESTS}'.lower() != 'true'
...                 SR-SIM 26.10 is required; enable with --variable 26_10_TESTS:true
Suite Teardown      Destroy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}


*** Variables ***
${lab-name}         srsim-new-chassis
${lab-file-name}    14-srsim-new-chassis.clab.yml
${runtime}          docker
${26_10_TESTS}    false


*** Test Cases ***
Deploy ${lab-name} lab
    Deploy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}

Check ixr-e2sc chassis
    Check chassis    ixr-e2sc    ixr-e2sc

Check ixr-e3 chassis
    Check chassis    ixr-e3    ixr-e3

Check sar-10x chassis
    Check chassis    sar-10x    sar-10x

Ensure new chassis containers stay running
    Sleep    30s    Let the simulators get past initial startup
    Check container is running    ixr-e2sc
    Check container is running    ixr-e3
    Check container is running    sar-10x

*** Keywords ***
Check chassis
    [Arguments]    ${node}    ${chassis}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect clab-${lab-name}-${node} --format '{{range .Config.Env}}{{println .}}{{end}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    NOKIA_SROS_CHASSIS=${chassis}

Check container is running
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect clab-${lab-name}-${node} --format '{{.State.Running}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${state} =    Strip String    ${output}
    Should Be Equal    ${state}    true
