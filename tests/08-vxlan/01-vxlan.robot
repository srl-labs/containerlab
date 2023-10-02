*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}         vxlan
${lab-file}         01-vxlan.clab.yml
${runtime}          docker
${vxlan-br}         clab-vxlan-br
${vxlan-br-ip}      172.20.25.1/24


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file} -d
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check VxLAN connectivity srl-linux
    Wait Until Keyword Succeeds    60    2s    Check VxLAN connectivity srl->linux

Check VxLAN connectivity linux-srl
    Wait Until Keyword Succeeds    60    2s    Check VxLAN connectivity linux->srl


*** Keywords ***
Check VxLAN connectivity srl->linux
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -it clab-vxlan-srl1 ip netns exec srbase-default ping 192.168.67.2 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Check VxLAN connectivity linux->srl
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec clab-vxlan-l2 ping 192.168.67.1 -c 1
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
    ...    sudo ip link set dev ${vxlan-br} up && sudo ip link set dev ${vxlan-br} mtu 9100 && sudo ip addr add ${vxlan-br-ip} dev ${vxlan-br}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip l del ${vxlan-br}
    Log    ${output}
