*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         comp-cfg-gen-test
${lab-file-name}    08-srsim-comp-cfg-test.clab.yml
${runtime}          docker
${gnmic_image}      ghcr.io/openconfig/gnmic:0.42.0
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

Check SR-2S power shelf configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr2s-a --path /configure/chassis[chassis-class=*][chassis-number=*]/power-shelf[power-shelf-id=*]/power-shelf-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain  ${output}    ps-a4-shelf-dc

Check SR-2S power module configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr2s-a --path /configure/chassis[chassis-class=*][chassis-number=*]/power-shelf[power-shelf-id=*]/power-module[power-module-id=*]/power-module-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain X Times   ${output}    ps-a-dc-6000  4

Check SR-2S card configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr2s-a --path /configure/card[slot-number=1]/card-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    xcm-2s

Check SR-2S xiom configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr2s-a --path /configure/card[slot-number=1]/xiom[xiom-slot=x1]/xiom-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    iom-s-3.0t

Check SR-2S MDA configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr2s-a --path /configure/card[slot-number=1]/xiom[xiom-slot=x1]/mda[mda-slot=1]/mda-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ms18-100gb-qsfp28

Ensure SR-2S MDA is up
    Wait Until Keyword Succeeds    2 minutes    10 seconds    Check SR-2S MDA state

Check SR-14S power shelf configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr14s-a --path /configure/chassis[chassis-class=*][chassis-number=*]/power-shelf[power-shelf-id=*]/power-shelf-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain X Times   ${output}    ps-a10-shelf-dc   2

Check SR-14S power module configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr14s-a --path /configure/chassis[chassis-class=*][chassis-number=*]/power-shelf[power-shelf-id=*]/power-module[power-module-id=*]/power-module-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain X Times   ${output}    ps-a-dc-6000  20

Check SR-14S card configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr14s-a --path /configure/card[slot-number=1]/card-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    xcm2-14s

Check SR-14S MDA configuration
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr14s-a --path /configure/card[slot-number=1]/mda[mda-slot=1]/mda-type
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    x2-s36-800g-qsfpdd-18.0t

Ensure SR-14S MDA is up
    Wait Until Keyword Succeeds    2 minutes    10 seconds    Check SR-14S MDA state

Check SR-2s-ncofg has unprovisioned card
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr2s-nocfg-a --path /state/card[slot-number=1]/hardware-data/oper-state
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    unprovisioned

# IPv4
Check secondary SR-14s inspect returns IPv4 mgmt IP correctly
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} ins -t ${lab-file-name} -f json | jq -r '.["comp-cfg-gen-test"][] | select(.name | contains("sr14s-2")) | .ipv4_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    172.20.20
    ${sr14s-2-ipv4} Set Variable    ${output}

Check secondary SR-14s slot 1 actually owns mgmt IPv4 addr
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["comp-cfg-gen-test"][] | select(.name | contains("sr14s-2-1")) | .ipv4_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${sr14s-2-ipv4}

Confirm SR-14s slot A doesn't have any IPv4 address
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["comp-cfg-gen-test"][] | select(.name | contains("sr14s-2-a")) | .ipv4_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    N/A

# IPv6
Check secondary SR-14s inspect returns IPv6 mgmt IP correctly
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} ins -t ${lab-file-name} -f json | jq -r '.["comp-cfg-gen-test"][] | select(.name | contains("sr14s-2")) | .ipv6_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    3fff:172:20:20
    ${sr14s-2-ipv6} Set Variable    ${output}

Check secondary SR-14s slot 1 actually owns mgmt IPv6 addr
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["comp-cfg-gen-test"][] | select(.name | contains("sr14s-2-1")) | .ipv6_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${sr14s-2-ipv6}

Confirm SR-14s slot A doesn't have any IPv6 address
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["comp-cfg-gen-test"][] | select(.name | contains("sr14s-2-a")) | .ipv6_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    N/A

*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup

Check SR-2S MDA state
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr2s-a --path /state/card[slot-number=1]/xiom[xiom-slot=x1]/mda[mda-slot=1]/hardware-data/oper-state
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    in-service

Check SR-14S MDA state
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --address clab-${lab-name}-sr14s-a --path /state/card[slot-number=1]/mda[mda-slot=1]/hardware-data/oper-state
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    in-service