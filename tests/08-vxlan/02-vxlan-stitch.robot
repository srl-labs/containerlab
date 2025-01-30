*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}         vxlan
${lab-file}         02-vxlan-stitch.clab.yml
${runtime}          docker
${lab-net}          clab-vxlan
${vxlan-br}         clab-vxlan-br
${vxlan-br-ip}      172.20.25.1/24


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file} -d
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check VxLAN interface parameters on the host for srl1 node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -d l show vx-srl1_e1-1

    Should Contain        ${output}    mtu 9050

    Should Contain        ${output}    vxlan id 100 remote 172.20.25.22 dev ${vxlan-br} srcport 0 0 dstport 14788
    
    Should Not Contain    ${output}    nolearning

Check veth interface parameters on the host for srl1 node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -d l show ve-srl1_e1-1

    Should Contain    ${output}    mtu 9500

    Should Contain    ${output}    link-netns clab-vxlan-stitch-srl1

Check VxLAN interface parameters on the host for very long name node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -d l show dev vx-some_very_long_node_name_l1_e1-1

    Should Contain    ${output}    mtu 9050

    Should Contain    ${output}    vxlan id 101 remote 172.20.25.23 dev clab-vxlan-br srcport 0 0 dstport 14789

Check veth interface parameters on the host for very long name node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -d l show dev ve-some_very_long_node_name_l1_e1-1

    Should Contain    ${output}    mtu 9500 qdisc noqueue state UP

    # in github actions the output for this link weirdly state the netnsid instead of nsname, thus we check for any of those
    Should Contain Any    ${output}    link-netns clab-vxlan-stitch-some_very_long_node_name_l1    link-netnsid 2

    Should Contain    ${output}    altname ve-some_very_long_node_name_l1_e1-1

Check VxLAN connectivity srl-linux
    # CI env var is set to true in Github Actions
    # and this test won't run there, since it fails for unknown reason
    IF    '%{CI=false}'=='false'
        Wait Until Keyword Succeeds    60    2s    Check VxLAN connectivity srl->linux
    END

Check VxLAN connectivity linux-srl
    IF    '%{CI=false}'=='false'
        Wait Until Keyword Succeeds    60    2s    Check VxLAN connectivity linux->srl
    END


*** Keywords ***
Check VxLAN connectivity srl->linux
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -it clab-vxlan-stitch-srl1 ip netns exec srbase-default ping 192.168.67.2 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Check VxLAN connectivity linux->srl
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec clab-vxlan-stitch-l2 ping 192.168.67.1 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Setup
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'
    # setup vxlan underlay bridge
    # we have to setup an underlay management bridge with big enought mtu to support vxlan and srl requirements for link mtu
    # we set mtu 9100 (and not the default 9500) because srl can't set vxlan mtu > 9412 and < 1500
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link add ${vxlan-br} type bridge || true
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link set dev ${vxlan-br} up && sudo ip link set dev ${vxlan-br} mtu 9100 && sudo ip addr add ${vxlan-br-ip} dev ${vxlan-br} || true
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip l del ${vxlan-br}
    Log    ${output}
