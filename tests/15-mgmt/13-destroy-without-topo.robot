*** Settings ***
Documentation       Destroy by lab name removes every management network when the topology file is gone.
Library             OperatingSystem
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${runtime}          docker
${lab-name}         mgmt13
${workdir}          /tmp/clab-tests/mgmt13
${topo}             ${workdir}/13-destroy-without-topo.clab.yml


*** Test Cases ***
Deploy lab with two management networks
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Network Driver Should Be    clab-mgmt13-main    bridge
    Network Driver Should Be    clab-mgmt13-oob    bridge

Forwarding rules are installed for both bridges
    ${main-br} =    Network Bridge    clab-mgmt13-main
    ${oob-br} =    Network Bridge    clab-mgmt13-oob
    Set Suite Variable    ${main-br}
    Set Suite Variable    ${oob-br}
    ${count} =    Containerlab Firewall Rule Count    ${main-br}
    Should Be True    ${count} > 0
    ${count} =    Containerlab Firewall Rule Count    ${oob-br}
    Should Be True    ${count} > 0

Destroy by name after the topology file is removed
    Remove File    ${topo}
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy --name ${lab-name} --cleanup

All management networks are removed
    Network Should Not Exist    clab-mgmt13-main
    Network Should Not Exist    clab-mgmt13-oob

Forwarding rules are removed for both bridges
    ${count} =    Containerlab Firewall Rule Count    ${main-br}
    Should Be Equal As Integers    ${count}    0
    ${count} =    Containerlab Firewall Rule Count    ${oob-br}
    Should Be Equal As Integers    ${count}    0


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    Multiple management networks require Docker.
    Create Directory    ${workdir}
    Copy File    ${CURDIR}/13-destroy-without-topo.clab.yml    ${topo}

Cleanup
    Run Keyword And Ignore Error
    ...    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy --name ${lab-name} --cleanup
    Run Keyword And Ignore Error
    ...    Command Should Succeed    docker network rm clab-mgmt13-main clab-mgmt13-oob
    Run Keyword And Ignore Error    Command Should Succeed    sudo rm -rf ${workdir}
