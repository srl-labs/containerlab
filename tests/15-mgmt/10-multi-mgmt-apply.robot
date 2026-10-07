*** Settings ***
Documentation       Reconciling nodes across several management networks with apply.
Library             OperatingSystem
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Run Keyword And Ignore Error    Destroy Lab    ${added-vars}


*** Variables ***
${runtime}          docker
${topo}             ${CURDIR}/10-multi-mgmt-apply.clab.yml
${initial-vars}     ${CURDIR}/10-multi-mgmt-apply.vars.initial.yml
${moved-vars}       ${CURDIR}/10-multi-mgmt-apply.vars.moved.yml
${added-vars}       ${CURDIR}/10-multi-mgmt-apply.vars.added.yml


*** Test Cases ***
Deploy nodes on two management networks
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo} --vars ${initial-vars}
    ${a} =    Node Address    clab-mgmt10-a    clab-mgmt10-main
    Address Should Be In Pool    ${a}    198.18.110.0/24
    ${b} =    Node Address    clab-mgmt10-b    clab-mgmt10-oob
    Address Should Be In Pool    ${b}    198.18.111.0/24
    ${id} =    Container Id    clab-mgmt10-b
    Set Suite Variable    ${b-id}    ${id}

Apply without changes keeps the nodes
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} apply -t ${topo} --vars ${initial-vars}
    ${id} =    Container Id    clab-mgmt10-b
    Should Be Equal    ${id}    ${b-id}

Apply moves a node to another management network
    ${output} =    Command Should Succeed
    ...    ${CLAB_BIN} --runtime ${runtime} apply -t ${topo} --vars ${moved-vars}
    Should Contain    ${output}    MgmtNet
    ${id} =    Container Id    clab-mgmt10-b
    Should Not Be Equal    ${id}    ${b-id}
    ${b} =    Node Address    clab-mgmt10-b    clab-mgmt10-main
    Address Should Be In Pool    ${b}    198.18.110.0/24
    ${networks} =    Command Should Succeed
    ...    docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' clab-mgmt10-b
    Should Not Contain    ${networks}    clab-mgmt10-oob

Apply adds a node on a new management network
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} apply -t ${topo} --vars ${added-vars}
    Network Driver Should Be    clab-mgmt10-new    bridge
    ${c} =    Node Address    clab-mgmt10-c    clab-mgmt10-new
    Address Should Be In Pool    ${c}    198.18.112.0/24

Destroy removes every management network including the emptied one
    Destroy Lab    ${added-vars}
    Network Should Not Exist    clab-mgmt10-main
    Network Should Not Exist    clab-mgmt10-oob
    Network Should Not Exist    clab-mgmt10-new


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    Multiple management networks require Docker.

Destroy Lab
    [Arguments]    ${vars}
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --vars ${vars} --cleanup

Container Id
    [Arguments]    ${container}
    ${id} =    Command Should Succeed    docker inspect -f '{{.Id}}' ${container}
    RETURN    ${id}
