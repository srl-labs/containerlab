*** Settings ***
Documentation       Single bridge management network with custom bridge, gateway, range and MTU.
Library             OperatingSystem
Library             Collections
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Run Keyword And Ignore Error
...                     Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup


*** Variables ***
${runtime}          docker
${topo}             ${CURDIR}/01-basic-mgmt.clab.yml
${network}          clab-mgmt01
${bridge}           clab-mgmt01-br


*** Test Cases ***
Deploy lab with a custom management network
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}

Runtime network uses the configured settings
    ${output} =    Command Should Succeed
    ...    docker network inspect -f '{{json .}}' ${network}
    ${net} =    Evaluate    json.loads($output)    modules=json
    Should Be Equal    ${net}[Driver]    bridge
    Should Be Equal    ${net}[Options][com.docker.network.bridge.name]    ${bridge}
    Should Be Equal    ${net}[Options][com.docker.network.driver.mtu]    1400
    ${pools} =    Evaluate    {p['Subnet']: p for p in $net['IPAM']['Config']}
    Should Be Equal    ${pools}[198.18.101.0/24][Gateway]    198.18.101.254
    Should Be Equal    ${pools}[198.18.101.0/24][IPRange]    198.18.101.128/25
    Dictionary Should Contain Key    ${pools}    fd00:101::/64

Bridge carries the gateway address
    ${output} =    Command Should Succeed    ip -br addr show dev ${bridge}
    Should Contain    ${output}    198.18.101.254/24

Nodes get addresses from the configured pools
    ${v4} =    Node Address    clab-mgmt01-dynamic    ${network}
    Address Should Be In Pool    ${v4}    198.18.101.128/25
    ${v6} =    Node Address    clab-mgmt01-dynamic    ${network}    GlobalIPv6Address
    Address Should Be In Pool    ${v6}    fd00:101::/64
    ${v4} =    Node Address    clab-mgmt01-static    ${network}
    Should Be Equal    ${v4}    198.18.101.140
    ${v6} =    Node Address    clab-mgmt01-static    ${network}    GlobalIPv6Address
    Should Be Equal    ${v6}    fd00:101::40

Nodes inherit the network MTU
    ${output} =    Command Should Succeed    docker exec clab-mgmt01-dynamic ip link show eth0
    Should Contain    ${output}    mtu 1400

Nodes are reachable from the host and each other
    Wait Until Keyword Succeeds    10s    1s    Command Should Succeed    ping -c 1 -W 1 198.18.101.140
    Wait Until Keyword Succeeds    10s    1s    Command Should Succeed    ping -6 -c 1 -W 1 fd00:101::40
    Wait Until Keyword Succeeds    10s    1s
    ...    Command Should Succeed    docker exec clab-mgmt01-dynamic ping -c 1 -W 1 198.18.101.140

External access rules are installed for the bridge
    ${count} =    Containerlab Firewall Rule Count    ${bridge}
    Should Be True    ${count} > 0    no containerlab forwarding rules for ${bridge}

Host mode node is not attached to the management network
    ${mode} =    Command Should Succeed    docker inspect -f '{{.HostConfig.NetworkMode}}' clab-mgmt01-hostmode
    Should Be Equal    ${mode}    host

Topology export describes the management network
    ${data} =    Get File    ${CURDIR}/clab-mgmt01/topology-data.json
    ${data} =    Evaluate    json.loads($data)    modules=json
    ${mgmt} =    Set Variable    ${data}[clab][config][mgmt]
    Should Be Equal    ${mgmt}[network]    ${network}
    Should Be Equal    ${mgmt}[bridge]    ${bridge}
    Should Be Equal    ${mgmt}[ipv4-subnet]    198.18.101.0/24
    Should Be Equal    ${data}[nodes][static][mgmt-net]    ${network}

Destroy removes the network, bridge and firewall rules
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    Network Should Not Exist    ${network}
    Command Should Fail    ip link show dev ${bridge}
    ${count} =    Containerlab Firewall Rule Count    ${bridge}
    Should Be Equal As Integers    ${count}    0


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    Inspects docker network settings.
