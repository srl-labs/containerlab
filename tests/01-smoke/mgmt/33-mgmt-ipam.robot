*** Settings ***
Library             OperatingSystem
Library             Collections
Resource            ../../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup
Test Setup          Should Be Empty    ${labs}
Test Teardown       Destroy Labs


*** Variables ***
${runtime}          docker
${topo}             ${CURDIR}/33-mgmt-ipam.clab.yml
${vars-dir}         ${CURDIR}/clab-smoke33-vars
${network}          clab-smoke33
${parent}           clab-smoke33
${peer-ns}          clab-smoke33-peer
${parent-created}   ${False}
${peer-created}     ${False}
@{labs}


*** Test Cases ***
Static conflicts warn for both families and retain configured addresses
    ${output} =    Deploy Lab    33-static    static=${True}
    ${warnings} =    Evaluate    $output.count('Duplicate static management address; retaining configured address')
    Should Be Equal As Integers    ${warnings}    2
    Should Contain    ${output}    node=r1
    Should Contain    ${output}    address=198.18.33.140
    Should Contain    ${output}    address=fd00:33::20
    Static Addresses Should Be Retained    33-static

Disabling DAD suppresses static conflict warnings
    ${output} =    Deploy Lab    33-static    dad=${False}    static=${True}
    Should Not Contain    ${output}    Duplicate static management address
    Static Addresses Should Be Retained    33-static

Runtime provider skips DAD even when it is enabled
    ${output} =    Deploy Lab    33-static    provider=runtime    static=${True}
    Should Not Contain    ${output}    Duplicate static management address
    Static Addresses Should Be Retained    33-static

Shared MACVLAN network reserves other labs addresses with DAD disabled
    Shared Network Preserves Unique Addresses    macvlan

Shared bridge network reserves other labs addresses with DAD disabled
    Shared Network Preserves Unique Addresses    bridge


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    MACVLAN management requires Docker.
    Directory Should Not Exist    ${vars-dir}
    Command Should Succeed    sudo ip link add ${parent} type dummy
    Set Suite Variable    ${parent-created}    ${True}
    Create Directory    ${vars-dir}
    Command Should Succeed    sudo ip link set ${parent} up
    Command Should Succeed    sudo ip addr add 198.18.33.1/24 dev ${parent}
    Command Should Succeed    sudo ip -6 addr add fd00:33::1/64 dev ${parent} nodad
    Command Should Succeed    sudo ip netns add ${peer-ns}
    Set Suite Variable    ${peer-created}    ${True}
    Command Should Succeed    sudo ip link add smoke33-peer link ${parent} type macvlan mode bridge
    Command Should Succeed    sudo ip link set smoke33-peer netns ${peer-ns}
    Command Should Succeed    sudo ip -n ${peer-ns} link set lo up
    Command Should Succeed    sudo ip -n ${peer-ns} addr add 198.18.33.140/24 dev smoke33-peer
    Command Should Succeed    sudo ip -n ${peer-ns} -6 addr add fd00:33::20/64 dev smoke33-peer nodad
    Command Should Succeed    sudo ip -n ${peer-ns} link set smoke33-peer up

Deploy Lab
    [Arguments]    ${name}    ${driver}=macvlan    ${provider}=containerlab    ${dad}=${True}    ${static}=${False}
    ${vars} =    Evaluate
    ...    json.dumps(dict(name=$name, driver=$driver, provider=$provider, dad=$dad, static=$static))
    ...    modules=json
    Create File    ${vars-dir}/${name}.json    ${vars}
    IF    $name not in $labs
        Append To List    ${labs}    ${name}
    END
    ${output} =    Command Should Succeed
    ...    ${CLAB_BIN} --runtime docker deploy -t ${topo} --vars ${vars-dir}/${name}.json
    ${output} =    Evaluate    ' '.join($output.split())
    RETURN    ${output}

Static Addresses Should Be Retained
    [Arguments]    ${name}
    ${ipv4}    ${ipv6} =    Node Addresses    clab-${name}-r1    ${network}
    Should Be Equal    ${ipv4}    198.18.33.140
    Should Be Equal    ${ipv6}    fd00:33::20
    ${output} =    Command Should Succeed    sudo ip -n ${peer-ns} -j addr show smoke33-peer
    ${addresses} =    Evaluate    [a['local'] for a in json.loads($output)[0]['addr_info']]    modules=json
    List Should Contain Value    ${addresses}    ${ipv4}
    List Should Contain Value    ${addresses}    ${ipv6}

Shared Network Preserves Unique Addresses
    [Arguments]    ${driver}
    Deploy Lab    33-shared-a    driver=${driver}    dad=${False}
    ${first} =    Inspect Network    ${network}
    Should Be Equal    ${first}[Driver]    ${driver}
    ${a4}    ${a6} =    Node Addresses    clab-33-shared-a-r1    ${network}
    Deploy Lab    33-shared-b    driver=${driver}    dad=${False}
    ${second} =    Inspect Network    ${network}
    Should Be Equal    ${first}[Id]    ${second}[Id]
    Length Should Be    ${second}[Containers]    2
    ${b4}    ${b6} =    Node Addresses    clab-33-shared-b-r1    ${network}
    Should Not Be Equal    ${a4}    ${b4}
    Should Not Be Equal    ${a6}    ${b6}
    Wait Until Keyword Succeeds    10s    1s
    ...    Command Should Succeed    docker exec clab-33-shared-a-r1 ping -c 1 -W 1 ${b4}
    Wait Until Keyword Succeeds    10s    1s
    ...    Command Should Succeed    docker exec clab-33-shared-a-r1 ping -6 -c 1 -W 1 ${b6}
    ${output} =    Command Should Succeed    docker inspect clab-33-shared-b-r1
    ${container-id} =    Evaluate    json.loads($output)[0]['Id']    modules=json
    Deploy Lab    33-shared-b    driver=${driver}    dad=${False}
    ${output} =    Command Should Succeed    docker inspect clab-33-shared-b-r1
    ${current-id} =    Evaluate    json.loads($output)[0]['Id']    modules=json
    Should Be Equal    ${container-id}    ${current-id}
    Destroy Lab    33-shared-a
    ${remaining} =    Inspect Network    ${network}
    Should Be Equal    ${first}[Id]    ${remaining}[Id]
    Length Should Be    ${remaining}[Containers]    1
    ${current4}    ${current6} =    Node Addresses    clab-33-shared-b-r1    ${network}
    Should Be Equal    ${b4}    ${current4}
    Should Be Equal    ${b6}    ${current6}

Destroy Lab
    [Arguments]    ${name}
    Command Should Succeed
    ...    ${CLAB_BIN} --runtime docker destroy -t ${topo} --vars ${vars-dir}/${name}.json --cleanup
    Remove Values From List    ${labs}    ${name}
    ${output} =    Command Should Succeed    docker ps -a --format '{{.Names}}'
    ${names} =    Evaluate    $output.splitlines()
    List Should Not Contain Value    ${names}    clab-${name}-r1

Destroy Labs
    ${remaining} =    Copy List    ${labs}
    FOR    ${name}    IN    @{remaining}
        Run Keyword And Continue On Failure    Destroy Lab    ${name}
    END
    IF    ${parent-created}
        Network Should Not Exist    ${network}
    END

Cleanup
    IF    not ${parent-created}
        RETURN
    END
    Run Keyword And Continue On Failure    Destroy Labs
    IF    ${peer-created}
        Run Keyword And Continue On Failure    Command Should Succeed    sudo ip netns del ${peer-ns}
    END
    Run Keyword And Continue On Failure    Command Should Succeed    sudo ip link del ${parent}
    Remove Directory    ${vars-dir}    recursive=True
    Interface Should Not Exist    ${parent}

Command Should Succeed
    [Arguments]    ${command}
    ${rc}    ${output} =    Run And Return Rc And Output    ${command} 2>&1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0    ${command}: ${output}
    RETURN    ${output}

Inspect Network
    [Arguments]    ${name}
    ${output} =    Command Should Succeed    docker network inspect ${name}
    ${info} =    Evaluate    json.loads($output)[0]    modules=json
    RETURN    ${info}

Node Addresses
    [Arguments]    ${node}    ${name}    ${addressing}=explicit
    ${output} =    Command Should Succeed    docker inspect ${node}
    ${info} =    Evaluate    json.loads($output)[0]    modules=json
    Should Be True    ${info}[State][Running]
    ${endpoint} =    Set Variable    ${info}[NetworkSettings][Networks][${name}]
    IF    '${addressing}' == 'explicit'
        Should Be Equal    ${endpoint}[IPAMConfig][IPv4Address]    ${endpoint}[IPAddress]
        Should Be Equal    ${endpoint}[IPAMConfig][IPv6Address]    ${endpoint}[GlobalIPv6Address]
    ELSE
        ${requested-addresses} =    Evaluate    $endpoint.get('IPAMConfig') or {}
        Should Be Empty    ${requested-addresses}
    END
    ${output} =    Command Should Succeed    sudo nsenter -t ${info}[State][Pid] -n ip -j addr show eth0
    ${addresses} =    Evaluate    [a['local'] for a in json.loads($output)[0]['addr_info']]    modules=json
    List Should Contain Value    ${addresses}    ${endpoint}[IPAddress]
    List Should Contain Value    ${addresses}    ${endpoint}[GlobalIPv6Address]
    RETURN    ${endpoint}[IPAddress]    ${endpoint}[GlobalIPv6Address]

Interface Should Not Exist
    [Arguments]    ${name}
    ${output} =    Command Should Succeed    ip -j link show
    ${names} =    Evaluate    [link['ifname'] for link in json.loads($output)]    modules=json
    List Should Not Contain Value    ${names}    ${name}

Network Should Not Exist
    [Arguments]    ${name}
    ${output} =    Command Should Succeed    docker network ls --format '{{.Name}}'
    ${names} =    Evaluate    $output.splitlines()
    List Should Not Contain Value    ${names}    ${name}
