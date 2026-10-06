*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         srsim-integrated
${lab-file-name}    13-srsim-integrated.clab.yml
${runtime}          docker
${gnmic_image}      ghcr.io/openconfig/gnmic:0.42.1
${gnmic_flags}      --username admin --password NokiaSros1! --values-only --insecure


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait for 45s
    Sleep    45s    Let everything fully provision & come up

Check IXR-E2N card override is passed to the container
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect clab-${lab-name}-ixr-e2n --format '{{range .Config.Env}}{{println .}}{{end}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    NOKIA_SROS_CARD=cpm-ixr-e2n/imm4-sfp+4-sfp+

Check IXR-E2N card configuration is not generated
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-ixr-e2n --path /configure/card[slot-number=1]/card-type
    Log    ${output}
    Should Not Contain    ${output}    imm4-sfp+4-sfp+

Check IXR-E2N card is unprovisioned
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-ixr-e2n --path /state/card[slot-number=1]/hardware-data/oper-state
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    unprovisioned

Check IXR-R6 standby CPM slot is set
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect clab-${lab-name}-ixr-r6-b --format '{{range .Config.Env}}{{println .}}{{end}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    NOKIA_SROS_SLOT=B

Check IXR-R6 card configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-ixr-r6 --path /configure/card[slot-number=1]/card-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    iom-ixr-r6

Check IXR-R6 MDA configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-ixr-r6 --path /configure/card[slot-number=1]/mda[mda-slot=1]/mda-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    m6-10g-sfp++1-100g-qsfp28

Ensure IXR-R6 CPM A is up
    Wait Until Keyword Succeeds    2 minutes    10 seconds    Check IXR-R6 CPM state    A

Ensure IXR-R6 CPM B is up
    Wait Until Keyword Succeeds    3 minutes    10 seconds    Check IXR-R6 CPM state    B


Check SR-1s card configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr-1s --path /configure/card[slot-number=1]/card-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    xcm-1s

Check SR-1s XIOM configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr-1s --path /configure/card[slot-number=1]/xiom[xiom-slot=x1]/xiom-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    iom-s-3.0t

Check SR-1s XIOM MDA configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr-1s --path /configure/card[slot-number=1]/xiom[xiom-slot=x1]/mda[mda-slot=1]/mda-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ms18-100gb-qsfp28

Ensure SR-1s XIOM MDA is up
    Wait Until Keyword Succeeds    2 minutes    10 seconds    Check SR-1s XIOM MDA state

*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup

Check IXR-R6 CPM state
    [Arguments]    ${slot}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-ixr-r6 --path /state/cpm[cpm-slot=${slot}]/hardware-data/oper-state
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    in-service

Check SR-1s XIOM MDA state
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr-1s --path /state/card[slot-number=1]/xiom[xiom-slot=x1]/mda[mda-slot=1]/hardware-data/oper-state
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    in-service
