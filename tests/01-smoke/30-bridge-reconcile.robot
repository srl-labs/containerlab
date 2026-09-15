*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown


*** Variables ***
${lab-name}         bridge-reconcile
${topo}              30-bridge-reconcile.clab.yml
${initial-vars}      30-bridge-reconcile.vars.initial.yml
${remap-vars}        30-bridge-reconcile.vars.remap.yml
${runtime}           docker
${bridge-name}       br-30-reconcile
${runtime-cli-exec}  docker exec


*** Test Cases ***
Initial bridge deployment
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${initial-vars}
    Should Be Equal As Integers    ${rc}    0
    Interface Should Exist    eth1
    Interface Should Exist    eth2
    Host Interface Should Be Attached To Bridge    e2
    Host Interface Should Be Attached To Bridge    e3

Bridge endpoint remap converges in one deploy
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${remap-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply summary
    Interface Should Exist    eth1
    Interface Should Exist    eth2
    Host Interface Should Be Attached To Bridge    e1
    Host Interface Should Be Attached To Bridge    e2
    Host Interface Should Not Exist    e3

Remapped bridge is idempotent
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo} --vars ${CURDIR}/${remap-vars}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    added links
    Should Not Contain    ${output}    deleted endpoints
    Host Interface Should Be Attached To Bridge    e1
    Host Interface Should Be Attached To Bridge    e2


*** Keywords ***
Setup
    IF    '${runtime}' == 'podman'
        Set Suite Variable    ${runtime-cli-exec}    sudo podman exec
    END
    Run Clab Command    destroy --name ${lab-name} --cleanup
    Run    sudo ip link del ${bridge-name}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link add name ${bridge-name} type bridge
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link set ${bridge-name} up
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Teardown
    Run Clab Command    destroy --name ${lab-name} --cleanup
    Run    sudo ip link del ${bridge-name}

Run Clab Command
    [Arguments]    ${args}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ${args} 2>&1
    Log    ${output}
    RETURN    ${rc}    ${output}

Interface Should Exist
    [Arguments]    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec} clab-${lab-name}-n1 ip link show ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${interface}

Host Interface Should Be Attached To Bridge
    [Arguments]    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    master ${bridge-name}

Host Interface Should Not Exist
    [Arguments]    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show ${interface}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0
