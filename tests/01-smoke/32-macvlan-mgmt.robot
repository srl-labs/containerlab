*** Settings ***
Library             OperatingSystem
Library             Collections
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${runtime}                     docker
${topo}                        ${CURDIR}/32-macvlan-mgmt.clab.yml
${inferred-topo}               ${CURDIR}/32-macvlan-mgmt-inferred.clab.yml
${network}                     clab-smoke32
${parent}                      clab-smoke32
${peer-ns}                     clab-smoke32-peer
${dynamic-node}                clab-32-macvlan-mgmt-dynamic
${static-node}                 clab-32-macvlan-mgmt-static
${parent-created}              ${False}
${peer-created}                ${False}
${replacement-verified}        ${False}


*** Test Cases ***
Deploy dual-stack MACVLAN management network
    Command Should Succeed    ${CLAB_BIN} --runtime docker deploy -t ${topo}
    ${info} =    Inspect Network    ${network}
    Should Be Equal    ${info}[Driver]    macvlan
    Should Be Equal    ${info}[Options][parent]    ${parent}
    Should Be Equal    ${info}[Options][macvlan_mode]    bridge
    ${ipv4}    ${ipv6} =    Node Addresses    ${dynamic-node}    ${network}
    ${in-pool} =    Evaluate    ipaddress.ip_address($ipv4) in ipaddress.ip_network('198.18.32.128/25')    modules=ipaddress
    Should Be True    ${in-pool}
    ${in-pool} =    Evaluate    ipaddress.ip_address($ipv6) in ipaddress.ip_network('fd00:32::/120')    modules=ipaddress
    Should Be True    ${in-pool}
    Should Not Be Equal    ${ipv4}    198.18.32.129
    Set Suite Variable    ${preferred-v4}    ${ipv4}
    Set Suite Variable    ${preferred-v6}    ${ipv6}
    ${static-v4}    ${static-v6} =    Node Addresses    ${static-node}    ${network}
    Should Be Equal    ${static-v4}    198.18.32.140
    Should Be Equal    ${static-v6}    fd00:32::20
    Should Not Be Equal    ${ipv4}    ${static-v4}
    Should Not Be Equal    ${ipv6}    ${static-v6}

MACVLAN nodes inherit MTU and communicate over both families
    ${output} =    Command Should Succeed    docker inspect ${dynamic-node}
    ${pid} =    Evaluate    json.loads($output)[0]['State']['Pid']    modules=json
    ${output} =    Command Should Succeed    sudo nsenter -t ${pid} -n ip -d link show eth0
    Should Contain    ${output}    macvlan mode bridge
    Should Contain    ${output}    mtu 1400
    Wait Until Keyword Succeeds    10s    1s
    ...    Command Should Succeed    docker exec ${dynamic-node} ping -c 1 -W 1 198.18.32.140
    Wait Until Keyword Succeeds    10s    1s
    ...    Command Should Succeed    docker exec ${dynamic-node} ping -6 -c 1 -W 1 fd00:32::20

Auxiliary interface provides host connectivity
    ${output} =    Command Should Succeed    ip -j route get ${preferred-v4}
    ${aux} =    Evaluate    json.loads($output)[0]['dev']    modules=json
    Should Not Be Equal    ${aux}    ${parent}
    ${output} =    Command Should Succeed    ip -d link show ${aux}
    Should Contain    ${output}    macvlan mode bridge
    Set Suite Variable    ${aux-interface}    ${aux}
    Command Should Succeed    ping -I 198.18.32.129 -c 1 -W 2 ${preferred-v4}

DAD replaces preferred addresses occupied by an external peer
    Command Should Succeed    ${CLAB_BIN} --runtime docker destroy -t ${topo}
    Interface Should Not Exist    ${aux-interface}
    TRY
        Command Should Succeed    sudo ip -n ${peer-ns} addr add ${preferred-v4}/24 dev eth0
        Command Should Succeed    sudo ip -n ${peer-ns} -6 addr add ${preferred-v6}/64 dev eth0 nodad
        Log DAD Peer State
        Command Should Succeed    ${CLAB_BIN} --debug --runtime docker deploy -t ${topo}
        Log DAD Peer State
        ${ipv4}    ${ipv6} =    Node Addresses    ${dynamic-node}    ${network}
        Should Not Be Equal    ${ipv4}    ${preferred-v4}
        Should Not Be Equal    ${ipv6}    ${preferred-v6}
        Wait Until Keyword Succeeds    10s    1s
        ...    Command Should Succeed    docker exec ${dynamic-node} ping -c 1 -W 1 ${preferred-v4}
        Wait Until Keyword Succeeds    10s    1s
        ...    Command Should Succeed    docker exec ${dynamic-node} ping -6 -c 1 -W 1 ${preferred-v6}
        Set Suite Variable    ${preferred-v4}    ${ipv4}
        Set Suite Variable    ${preferred-v6}    ${ipv6}
        Set Suite Variable    ${replacement-verified}    ${True}
    FINALLY
        Command Should Succeed    sudo ip -n ${peer-ns} addr flush dev eth0 scope global
    END

Redeployment retains the replacement addresses after the conflict disappears
    Skip If    not ${replacement-verified}    DAD replacement did not complete successfully.
    Command Should Succeed    ${CLAB_BIN} --runtime docker destroy -t ${topo}
    Network Should Not Exist    ${network}
    Command Should Succeed    ${CLAB_BIN} --runtime docker deploy -t ${topo}
    ${ipv4}    ${ipv6} =    Node Addresses    ${dynamic-node}    ${network}
    Should Be Equal    ${ipv4}    ${preferred-v4}
    Should Be Equal    ${ipv6}    ${preferred-v6}
    Command Should Succeed    ${CLAB_BIN} --runtime docker destroy -t ${topo} --cleanup
    Network Should Not Exist    ${network}

Runtime IPAM infers IPv4 and IPv6 subnets from the parent
    Command Should Succeed    ${CLAB_BIN} --runtime docker destroy -t ${topo} --cleanup
    Network Should Not Exist    ${network}
    Command Should Succeed    ${CLAB_BIN} --runtime docker deploy -t ${inferred-topo}
    ${info} =    Inspect Network    clab-smoke32-inferred
    ${subnets} =    Evaluate    [pool['Subnet'] for pool in $info['IPAM']['Config']]
    List Should Contain Value    ${subnets}    198.18.32.0/24
    List Should Contain Value    ${subnets}    fd00:32::/64
    ${ipv4}    ${ipv6} =    Node Addresses    clab-32-macvlan-inferred-n1    clab-smoke32-inferred    runtime
    ${in-subnet} =    Evaluate    ipaddress.ip_address($ipv4) in ipaddress.ip_network('198.18.32.0/24')    modules=ipaddress
    Should Be True    ${in-subnet}
    ${in-subnet} =    Evaluate    ipaddress.ip_address($ipv6) in ipaddress.ip_network('fd00:32::/64')    modules=ipaddress
    Should Be True    ${in-subnet}
    Command Should Succeed    ${CLAB_BIN} --runtime docker destroy -t ${inferred-topo} --cleanup

Reject inferred IPv4 /31 before creating the network
    Command Should Succeed    sudo ip addr flush dev ${parent} scope global
    Command Should Succeed    sudo ip addr add 198.18.32.0/31 dev ${parent}
    Inferred Deployment Should Fail

Reject inferred IPv6 /127 before creating the network
    Command Should Succeed    sudo ip addr flush dev ${parent} scope global
    Command Should Succeed    sudo ip -6 addr add fd00:32::1/127 dev ${parent} nodad
    Inferred Deployment Should Fail


*** Keywords ***
Log DAD Peer State
    Command Should Succeed    uname -r
    Command Should Succeed    sudo ip -n ${peer-ns} -d -s addr show dev eth0
    Command Should Succeed    sudo ip -n ${peer-ns} -4 route show table local
    Command Should Succeed
    ...    sudo ip netns exec ${peer-ns} sysctl net.ipv4.conf.all.arp_ignore net.ipv4.conf.eth0.arp_ignore net.ipv4.conf.all.arp_filter net.ipv4.conf.eth0.arp_filter

Setup
    Skip If    '${runtime}' != 'docker'    MACVLAN management requires Docker.
    Command Should Succeed    sudo ip netns add ${peer-ns}
    Set Suite Variable    ${peer-created}    ${True}
    Command Should Succeed    sudo ip link add ${parent} type veth peer name eth0 netns ${peer-ns}
    Set Suite Variable    ${parent-created}    ${True}
    Command Should Succeed    sudo ip -n ${peer-ns} link set lo up
    ${sysctls} =    Catenate
    ...    net.ipv4.conf.all.arp_ignore=0 net.ipv4.conf.eth0.arp_ignore=0
    ...    net.ipv4.conf.all.arp_filter=0 net.ipv4.conf.eth0.arp_filter=0
    ...    net.ipv4.conf.all.rp_filter=0 net.ipv4.conf.eth0.rp_filter=0
    ...    net.ipv6.conf.eth0.disable_ipv6=0
    Command Should Succeed    sudo ip netns exec ${peer-ns} sysctl -w ${sysctls}
    Command Should Succeed    sudo ip -n ${peer-ns} link set eth0 mtu 1400 up
    Command Should Succeed    sudo ip link set ${parent} mtu 1400 up
    Command Should Succeed    sudo ip addr add 198.18.32.1/24 dev ${parent}
    Command Should Succeed    sudo ip -6 addr add fd00:32::1/64 dev ${parent} nodad

Cleanup
    IF    ${parent-created}
        Run Keyword And Continue On Failure
        ...    Command Should Succeed    ${CLAB_BIN} --runtime docker destroy -t ${topo} --cleanup
        Run Keyword And Continue On Failure
        ...    Command Should Succeed    ${CLAB_BIN} --runtime docker destroy -t ${inferred-topo} --cleanup
        Run Keyword And Continue On Failure    Command Should Succeed    sudo ip link del ${parent}
        Run Keyword And Continue On Failure    Network Should Not Exist    ${network}
        Run Keyword And Continue On Failure    Network Should Not Exist    clab-smoke32-inferred
    END
    IF    ${peer-created}
        Run Keyword And Continue On Failure    Command Should Succeed    sudo ip netns del ${peer-ns}
    END

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

Inferred Deployment Should Fail
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime docker deploy -t ${inferred-topo} 2>&1
    Should Not Be Equal As Integers    ${rc}    0
    ${output} =    Evaluate    ' '.join($output.split())
    Should Contain    ${output}    prefix must contain at least four addresses
    Network Should Not Exist    clab-smoke32-inferred
