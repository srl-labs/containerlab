*** Comments ***
This test suite verifies
- connectivity of nodes to the linux bridge
- connectivity of nodes to the host netns
- user-specified bridge is honored as a mgmt net bridge


*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}             bridge-and-host
${lab-file}             03-linux-nodes-to-bridge-and-host.clab.yml
${bridge-name}          br-01-03-clab
${br-link1-name}        l1-eth1
${br-link2-name}        l1-eth2
${host-link-name}       l1-01-03-eth3
${runtime}              docker
${mgmt-br-name}         01-03-mgmt


*** Test Cases ***
Create linux bridge
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link add name ${bridge-name} type bridge && sudo ip link set ${bridge-name} up
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy --skip-labdir-acl -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in linux bridge
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${br-link1-name}
    Log    ${output}
    Should Contain    ${output}    master ${bridge-name} state UP
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${br-link2-name}
    Log    ${output}
    Should Contain    ${output}    master ${bridge-name} state UP

Verify links in host ns
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${host-link-name}
    Log    ${output}
    Should Contain    ${output}    state UP

Verify management network is using user-specified bridge
    # show management interface info and cut the information about the ifindex of the remote veth
    # note that exec returns the info in the stderr stream, thus we use stderr to parse the ifindex
    ${rc}    ${iface} =    OperatingSystem.Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label clab-node-name\=l1 --cmd "ip l show eth0" 2>&1 | perl -lne '/.*[0-9]+: .*\\@if(.*:) .*/ && print $1'
    Log    ${iface}
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${res} =    OperatingSystem.Run And Return Rc And Output
    ...    sudo ip l | grep "${iface}"
    Log    ${res}
    Should Contain    ${res}    master ${mgmt-br-name} state UP

Verify iptables allow rule is set
    [Documentation]    Checking if iptables allow rule is set so that external traffic can reach containerlab management network
    Skip If    '${runtime}' != 'docker'

    ${ipt} =    Run
    ...    sudo iptables -vnL FORWARD
    Log    ${ipt}
    # debian 12 uses `0` for protocol, while previous versions use `all`
    # this matches the rule in the in direction
    Should Contain Any    ${ipt}
    ...    ACCEPT all -- * ${bridge-name}
    ...    ACCEPT 0 -- * ${bridge-name}
    ...    ignore_case=True
    ...    collapse_spaces=True

    # this matches the rule in the out direction
    Should Contain Any    ${ipt}
    ...    ACCEPT all -- ${bridge-name} *
    ...    ACCEPT 0 -- ${bridge-name} *
    ...    ignore_case=True
    ...    collapse_spaces=True

Verify ip6tables allow rule is set
    [Documentation]    Checking if ip6tables allow rule is set so that external traffic can reach containerlab management network
    Skip If    '${runtime}' != 'docker'

    # Add check for ip6tables availability
    ${rc}    ${output} =    Run And Return Rc And Output    which nft
    Skip If    ${rc} != 0    nft command not found

    ${rc}    ${output} =    Run And Return Rc And Output    sudo nft list tables
    Skip If    'ip6 filter' not in '''${output}'''    ip6 filter chain not found

    ${ipt} =    Run
    ...    sudo nft list chain ip6 filter FORWARD
    Log    ${ipt}
    Should Match Regexp    ${ipt}    oifname.*${bridge-name}.*accept
    Should Match Regexp    ${ipt}    iifname.*${bridge-name}.*accept

*** Keywords ***
Setup
    # ensure the bridge we about to create is deleted first
    Run    sudo ip l del ${bridge-name}
    # remove the alpine:3 container image, to test that we are able to live-pull it
    Run    sudo docker image rm alpine:3
    Run    sudo ctr -n clab image rm docker.io/library/alpine:3

Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    Run    sudo ip l del ${bridge-name}
    Run    sudo ip l del ${host-link-name}
