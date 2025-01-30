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
${lab-net}          clab-vxlan
${vxlan-br}         clab-vxlan-br
${vxlan-br-ip}      172.20.25.1/24


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file} -d
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

# This test started to fail once we upgraded to srl 24.7.1 with the following error:
# TODO: investigate why this test started to fail, why the interface is becoming monit_in and not the ifindex of the bridge that the vxlan packets are routed through
# Check VxLAN interface parameters in srl node    | FAIL |
# '9: e1-1@if9: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 9050 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
#    link/ether 1a:6f:01:ff:00:01 brd ff:ff:ff:ff:ff:ff link-netnsid 0 promiscuity 0    allmulti 0 minmtu 68 maxmtu 65535
#    vxlan id 100 remote 172.20.25.22 dev monit_in srcport 0 0 dstport 14788 ttl auto ageing 300 udpcsum noudp6zerocsumtx noudp6zerocsumrx addrgenmode eui64 numtxqueues 1 numrxqueues 1 gso_max_size 65536 gso_max_segs 65535 tso_max_size 524280 tso_max_segs 65535 gro_max_size 65536 ' does not contain 'vxlan id 100 remote 172.20.25.22 dev if4 srcport 0 0 dstport 14788'
# Check VxLAN interface parameters in srl node
    # the commented out piece is to identify the link ifindex for a clab network
    # but since we use a custom network here, we can just use its name, as the link will **not** be in the form of br-<id>
    # ...    sudo docker inspect -f '{{.Id}}' ${lab-net} | cut -c1-12 | xargs echo br- | tr -d ' ' | xargs ip -j l show | jq -r '.[0].ifindex'
    # ${rc}    ${link_ifindex} =    Run And Return Rc And Output
    # ...    ip -j l show ${vxlan-br} | jq -r '.[0].ifindex'

    # Log    ${link_ifindex}

    # ${rc}    ${output} =    Run And Return Rc And Output
    # ...    sudo docker exec clab-${lab-name}-srl1 ip -d l show e1-1

    # Should Contain    ${output}    vxlan id 100 remote 172.20.25.22 dev if${link_ifindex} srcport 0 0 dstport 14788

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
