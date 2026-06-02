*** Comments ***
This test suite verifies
- the management bridge is not deleted when --keep-mgmt-net is present and the lab is destroyed
- the management bridge is deleted by default


*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup


*** Variables ***
${lab-name}         7-keep-mgmt-net
${topo}             ${CURDIR}/07-linux-single-node.clab.yml
${mgmt-bridge}      01-07-net


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Destroy ${lab-name} lab keep mgmt net
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --keep-mgmt-net
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check ${lab-name} mgmt network remains
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip l show dev ${mgmt-bridge}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Deploy ${lab-name} lab again
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Destroy ${lab-name} lab dont keep mgmt net
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check ${lab-name} mgmt network is gone
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip l show dev ${mgmt-bridge}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0


*** Keywords ***
Setup
    # skipping this test suite for podman as keep-mgmt-net fails with podman for now
    Skip If    '${runtime}' == 'podman'
