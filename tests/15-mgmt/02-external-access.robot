*** Settings ***
Documentation       Management network with external access disabled.
Library             OperatingSystem
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Run Keyword And Ignore Error
...                     Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup


*** Variables ***
${runtime}          docker
${topo}             ${CURDIR}/02-external-access.clab.yml
${network}          clab-mgmt02
${bridge}           clab-mgmt02-br


*** Test Cases ***
Deploy lab with external access disabled
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}

No forwarding rules are installed for the bridge
    ${count} =    Containerlab Firewall Rule Count    ${bridge}
    Should Be Equal As Integers    ${count}    0

Node is still reachable from the host
    ${v4} =    Node Address    clab-mgmt02-n1    ${network}
    Address Should Be In Pool    ${v4}    198.18.102.0/24
    Wait Until Keyword Succeeds    10s    1s    Command Should Succeed    ping -c 1 -W 1 ${v4}

Destroy removes the network
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    Network Should Not Exist    ${network}


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    Inspects docker firewall rules.
