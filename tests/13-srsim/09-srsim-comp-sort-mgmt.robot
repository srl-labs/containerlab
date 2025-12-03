*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         comp-sort-mgmt
${lab-file-name}    09-srsim-comp-sort-mgmt.clab.yml
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

# IPv4
Check SR-14s inspect returns IPv4 mgmt IP correctly
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ins -t ${CURDIR}/${lab-file-name} -f json | jq -r '.["${lab-name}"][] | select(.name | contains("sr14s")) | .ipv4_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    172.20.20
    Set Suite Variable    ${sr14s-ipv4}    ${output}

Check SR-14s slot 1 actually owns mgmt IPv4 addr
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["${lab-name}"][] | select(.name | contains("sr14s-1")) | .ipv4_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${sr14s-ipv4}

Confirm SR-14s slot A doesn't have any IPv4 address
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["${lab-name}"][] | select(.name | contains("sr14s-a")) | .ipv4_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    N/A

# IPv6
Check SR-14s inspect returns IPv6 mgmt IP correctly
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ins -t ${CURDIR}/${lab-file-name} -f json | jq -r '.["${lab-name}"][] | select(.name | contains("sr14s")) | .ipv6_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    3fff:172:20:20
    Set Suite Variable    ${sr14s-ipv6}    ${output}

Check SR-14s slot 1 actually owns mgmt IPv6 addr
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["${lab-name}"][] | select(.name | contains("sr14s-1")) | .ipv6_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${sr14s-ipv6}

Confirm SR-14s slot A doesn't have any IPv6 address
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["${lab-name}"][] | select(.name | contains("sr14s-a")) | .ipv6_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    N/A

*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup