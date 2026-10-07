*** Settings ***
Library             OperatingSystem
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${runtime}              docker
${topo}                 ${CURDIR}/09-multi-mgmt.clab.yml
${parent}               clab-smoke35
${uplink}               clab-smoke35-u
${parent-created}       ${False}


*** Test Cases ***
Deploy lab with bridge, runtime IPAM and macvlan management networks
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Network Driver Should Be    clab-smoke35-main    bridge
    Network Driver Should Be    clab-smoke35-auto    bridge
    Network Driver Should Be    clab-smoke35-mv    macvlan

Bridge network with containerlab IPAM assigns dual-stack addresses
    ${v4} =    Node Address    clab-smoke35-r1    clab-smoke35-main    IPAddress
    Address Should Be In Pool    ${v4}    198.18.35.0/24
    ${v6} =    Node Address    clab-smoke35-r1    clab-smoke35-main    GlobalIPv6Address
    Address Should Be In Pool    ${v6}    fd00:35::/64
    Wait Until Keyword Succeeds    10s    1s    Command Should Succeed    ping -c 1 -W 1 ${v4}

Bridge network with runtime IPAM uses the runtime subnet
    ${subnet} =    Command Should Succeed
    ...    docker network inspect -f '{{(index .IPAM.Config 0).Subnet}}' clab-smoke35-auto
    ${v4} =    Node Address    clab-smoke35-r2    clab-smoke35-auto    IPAddress
    Address Should Be In Pool    ${v4}    ${subnet}
    Wait Until Keyword Succeeds    10s    1s    Command Should Succeed    ping -c 1 -W 1 ${v4}

Macvlan network assigns dynamic and static addresses reachable from the host
    ${dynamic} =    Node Address    clab-smoke35-r3    clab-smoke35-mv    IPAddress
    Address Should Be In Pool    ${dynamic}    198.18.135.128/25
    ${static} =    Node Address    clab-smoke35-r4    clab-smoke35-mv    IPAddress
    Should Be Equal    ${static}    198.18.135.140
    Wait Until Keyword Succeeds    10s    1s    Command Should Succeed    ping -c 1 -W 1 ${dynamic}
    Wait Until Keyword Succeeds    10s    1s    Command Should Succeed    ping -c 1 -W 1 ${static}

Host mode node does not need a management network
    ${mode} =    Command Should Succeed    docker inspect -f '{{.HostConfig.NetworkMode}}' clab-smoke35-h1
    Should Be Equal    ${mode}    host

Destroy removes all management networks
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    ${output} =    Command Should Succeed    docker network ls --format '{{.Name}}'
    Should Not Contain    ${output}    clab-smoke35-


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    MACVLAN management requires Docker.
    Command Should Succeed    sudo ip link add ${parent} type bridge
    Set Suite Variable    ${parent-created}    ${True}
    Command Should Succeed    sudo ip link set ${parent} up
    # a bridge without ports has no carrier on some kernels, which keeps macvlan children down
    Command Should Succeed    sudo ip link add ${uplink} type dummy
    Command Should Succeed    sudo ip link set ${uplink} master ${parent} up
    Command Should Succeed    sudo ip addr add 198.18.135.1/24 dev ${parent}

Cleanup
    Run Keyword And Ignore Error
    ...    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    IF    ${parent-created}
        Run Keyword And Continue On Failure    Command Should Succeed    sudo ip link del ${uplink}
        Run Keyword And Continue On Failure    Command Should Succeed    sudo ip link del ${parent}
    END

