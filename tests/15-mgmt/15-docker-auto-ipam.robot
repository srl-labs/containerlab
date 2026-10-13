*** Settings ***
Documentation       Docker-selected subnets must support deployment with the default IPAM provider.
Library             OperatingSystem
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Cleanup
Test Teardown       Destroy Lab


*** Variables ***
${runtime}          docker
${topo}             ${CURDIR}/15-docker-auto-ipam.clab.yml
${vars-dir}         ${CURDIR}/clab-auto-ipam-vars
${name}             ${EMPTY}
${network}          ${EMPTY}
${network-flag}     ${EMPTY}


*** Test Cases ***
Fresh auto subnet with mapping notation
    Prepare Lab    mapping    auto
    Deploy And Check Addresses

Fresh auto subnet with list notation
    Prepare Lab    list    auto    list=${True}
    Deploy And Check Addresses

Fresh dual-stack auto subnets
    Prepare Lab    dual    auto    auto
    Deploy And Check Addresses

IPv6 auto with omitted IPv4 subnet
    Prepare Lab    ipv6    ${EMPTY}    auto
    Deploy And Check Addresses

Subnetless additional management network
    Prepare Lab    extra    ${EMPTY}    list=${True}    extra=${True}
    Deploy And Check Addresses

Existing Docker-selected subnet with auto topology
    Prepare Lab    reused-auto    auto
    Command Should Succeed    docker network create ${network}
    Deploy And Check Addresses

Default management settings reuse a Docker-selected subnet
    Prepare Lab    reused-default    default=${True}
    Command Should Succeed    docker network create ${network}
    Deploy And Check Addresses

Explicit IPv6 address survives fallback on Docker-selected IPv4 subnet
    Prepare Lab    mixed    auto    fd00:217::/64    static6=fd00:217::10
    Deploy And Check Addresses
    ${ipv6} =    Node Address    clab-${name}-r1    ${network}    GlobalIPv6Address
    Should Be Equal    ${ipv6}    fd00:217::10

Explicit address on Docker-selected subnet is not silently reassigned
    Prepare Lab    static-rejected    auto
    Command Should Succeed    docker network create ${network}
    ${pools} =    Command Should Succeed    docker network inspect -f '{{json .IPAM.Config}}' ${network}
    ${ipv4} =    Evaluate
    ...    str(ipaddress.ip_network(json.loads($pools)[0]['Subnet']).network_address + 10)
    ...    modules=ipaddress,json
    ${vars} =    Get File    ${vars-dir}/${name}.json
    ${vars} =    Evaluate    json.dumps(dict(json.loads($vars), StaticIPv4=$ipv4))    modules=json
    Create File    ${vars-dir}/${name}.json    ${vars}
    ${output} =    Command Should Fail
    ...    ${CLAB_BIN} --runtime docker deploy -t ${topo} --vars ${vars-dir}/${name}.json
    Should Contain    ${output}    user specified IP address is supported only

Explicit subnets retain containerlab address allocation
    Prepare Lab    explicit    198.18.215.0/24    fd00:215::/64
    ${output} =    Deploy And Check Addresses
    Should Not Contain    ${output}    Using runtime IPAM
    ${ipv4} =    Node Address    clab-${name}-r1    ${network}
    ${requested4} =    Node Address    clab-${name}-r1    ${network}    IPAMConfig.IPv4Address
    Should Be Equal    ${ipv4}    ${requested4}
    ${ipv6} =    Node Address    clab-${name}-r1    ${network}    GlobalIPv6Address
    ${requested6} =    Node Address    clab-${name}-r1    ${network}    IPAMConfig.IPv6Address
    Should Be Equal    ${ipv6}    ${requested6}


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    This suite tests Docker IPAM behavior.
    Directory Should Not Exist    ${vars-dir}
    Create Directory    ${vars-dir}

Prepare Lab
    [Arguments]    ${case}    ${ipv4}=${EMPTY}    ${ipv6}=${EMPTY}
    ...    ${list}=${False}    ${extra}=${False}    ${default}=${False}
    ...    ${static6}=${EMPTY}
    ${lab-name} =    Set Variable    auto-ipam-${case}
    ${lab-network} =    Set Variable    clab-${lab-name}-mgmt
    Network Should Not Exist    ${lab-network}
    Set Suite Variable    ${name}    ${lab-name}
    Set Suite Variable    ${network}    ${lab-network}
    ${mgmt} =    Evaluate
    ...    dict(network=$network, **({'ipv4-subnet': $ipv4} if $ipv4 else {}), **({'ipv6-subnet': $ipv6} if $ipv6 else {}))
    IF    ${extra}
        ${mgmt} =    Evaluate    [{'network': $network + '-main', 'ipv4-subnet': '198.18.216.0/24'}, $mgmt]
    ELSE IF    ${list}
        ${mgmt} =    Create List    ${mgmt}
    END
    ${vars} =    Evaluate
    ...    json.dumps(dict(Name=$name, Network=$network, Mgmt='' if $default else json.dumps($mgmt), StaticIPv4='', StaticIPv6=$static6))
    ...    modules=json
    Create File    ${vars-dir}/${name}.json    ${vars}
    Set Suite Variable    ${network-flag}    ${EMPTY}
    IF    ${default}
        Set Suite Variable    ${network-flag}    --network ${network}
    END

Deploy And Check Addresses
    ${output} =    Command Should Succeed
    ...    ${CLAB_BIN} --runtime docker deploy -t ${topo} --vars ${vars-dir}/${name}.json ${network-flag}
    Should Not Contain    ${output}    user specified IP address is supported only
    ${pools} =    Command Should Succeed    docker network inspect -f '{{json .IPAM.Config}}' ${network}
    ${pools} =    Evaluate    json.loads($pools)    modules=json
    FOR    ${pool}    IN    @{pools}
        ${field} =    Evaluate    'IPAddress' if ':' not in $pool['Subnet'] else 'GlobalIPv6Address'
        ${a} =    Node Address    clab-${name}-r1    ${network}    ${field}
        ${b} =    Node Address    clab-${name}-r2    ${network}    ${field}
        Address Should Be In Pool    ${a}    ${pool}[Subnet]
        Address Should Be In Pool    ${b}    ${pool}[Subnet]
        Should Not Be Equal    ${a}    ${b}
    END
    FOR    ${node}    IN    r1    r2
        ${state} =    Command Should Succeed    docker inspect -f '{{.State.Status}}' clab-${name}-${node}
        Should Be Equal    ${state}    running
    END
    RETURN    ${output}

Destroy Lab
    IF    '${name}' == ''
        RETURN
    END
    IF    '${network-flag}' != ''
        ${vars} =    Get File    ${vars-dir}/${name}.json
        ${vars} =    Evaluate
        ...    json.dumps(dict(json.loads($vars), Mgmt=json.dumps(dict(network=$network))))
        ...    modules=json
        Create File    ${vars-dir}/${name}.json    ${vars}
    END
    Run Keyword And Continue On Failure
    ...    Command Should Succeed
    ...    ${CLAB_BIN} --runtime docker destroy -t ${topo} --vars ${vars-dir}/${name}.json --cleanup
    Run Keyword And Ignore Error    Command Should Succeed    docker network rm ${network}
    Run Keyword And Ignore Error    Command Should Succeed    docker network rm ${network}-main
    Set Suite Variable    ${name}    ${EMPTY}

Cleanup
    Run Keyword And Continue On Failure    Destroy Lab
    Remove Directory    ${vars-dir}    recursive=True
