*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot
Resource            ../srsim.robot

Suite Setup         Skip If    '${26_10_TESTS}'.lower() != 'true'
...                 SR-SIM 26.10 is required; enable with --variable 26_10_TESTS:true
Suite Teardown      Destroy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}


*** Variables ***
${lab-name}         srsim-new-chassis
${lab-file-name}    14-srsim-new-chassis.clab.yml
${runtime}          docker
${26_10_TESTS}      false
${gnmic_image}      ghcr.io/openconfig/gnmic:0.42.1
${gnmic_flags}      --username admin --password NokiaSros1! --values-only --insecure


*** Test Cases ***
Deploy ${lab-name} lab
    Deploy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}

Check ixr-e2sc chassis
    Check chassis    ixr-e2sc    ixr-e2sc

Check ixr-e3 chassis
    Check chassis    ixr-e3    ixr-e3

Check sar-10x chassis
    Check chassis    sar-10x-a    sar-10x
    Check chassis    sar-10x-b    sar-10x

Ensure new chassis containers stay running
    Sleep    30s    Let the simulators get past initial startup
    Check container is running    ixr-e2sc
    Check container is running    ixr-e3
    Check container is running    sar-10x-a
    Check container is running    sar-10x-b

Check sar-10x line cards are up
    Wait Until Keyword Succeeds    3 minutes    15 seconds    SAR-10x line cards are up

*** Keywords ***
SAR-10x line cards are up
    # Query card 1, MDAs 1/1..1/8, then card 2 in chassis order.
    ${command} =    Catenate    SEPARATOR=${SPACE}
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags}
    ...    --address clab-${lab-name}-sar-10x
    ...    --path /state/card[slot-number=1]/hardware-data/oper-state
    ...    --path /state/card[slot-number=1]/mda[mda-slot=1]/hardware-data/oper-state
    ...    --path /state/card[slot-number=1]/mda[mda-slot=2]/hardware-data/oper-state
    ...    --path /state/card[slot-number=1]/mda[mda-slot=3]/hardware-data/oper-state
    ...    --path /state/card[slot-number=1]/mda[mda-slot=4]/hardware-data/oper-state
    ...    --path /state/card[slot-number=1]/mda[mda-slot=5]/hardware-data/oper-state
    ...    --path /state/card[slot-number=1]/mda[mda-slot=6]/hardware-data/oper-state
    ...    --path /state/card[slot-number=1]/mda[mda-slot=7]/hardware-data/oper-state
    ...    --path /state/card[slot-number=1]/mda[mda-slot=8]/hardware-data/oper-state
    ...    --path /state/card[slot-number=2]/hardware-data/oper-state
    ${rc}    ${output} =    Run And Return Rc And Output    ${command}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${up_states} =    Get Regexp Matches    ${output}    in-service
    Length Should Be    ${up_states}    10
    Should Not Contain    ${output}    null

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
