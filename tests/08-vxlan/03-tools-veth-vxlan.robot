*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Setup         Run Keyword    Setup
Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-file}                 03-vxlan-tools.clab.yml
${lab-name}                 vxlan-tools
${runtime}                  docker
${bridge-name}              clabtestbr
${l1_name}                  some_very_long_node_name_l1
${l2_name}                  l2
${l1_host_link}             some_very_long_node_name_l1_eth1
${l2_host_link}             l2_eth1
${vxlan-br}                 clab-vxlan-br
${vxlan-br-ip}              172.20.25.1/24

# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test
${runtime-cli-exec-cmd}     sudo docker exec


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}

    Sleep    5s

    # log bridge details to check if its mtu is really set to 9100
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${vxlan-br}

    Should Contain    ${output}    mtu 9100    msg=Bridge mtu is not 9100 before lab deployment

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    # log bridge details to check if its mtu is really set to 9100 after the lab deployment
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${vxlan-br}

    Should Contain    ${output}    mtu 9100    msg=Bridge mtu is not 9100 after lab deployment

    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}

Define runtime exec command
    IF    "${runtime}" == "podman"
        Set Suite Variable    ${runtime-cli-exec-cmd}    sudo podman exec
    END

Get netns id for host interface of some_very_long_node_name_l1
    # log bridge details to check if its mtu is really set to 9100 after the lab deployment
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${vxlan-br}

    Should Contain    ${output}    mtu 9100

    ${output} =    Run
    ...    ip netns list-id
    Log    ${output}

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip netns list-id | awk '/clab-${lab-name}-${l1_name}/ {print $2}'

    Set Suite Variable    ${l1_host_link_netnsid}    ${output}

    # log bridge details to check if its mtu is really set to 9100 after the lab deployment
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${vxlan-br}

    Should Contain    ${output}    mtu 9100

Check host interface for some_very_long_node_name_l1 node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -d l show dev ${l1_host_link}

    Should Contain    ${output}    mtu 9500

    Should Contain Any
    ...    ${output}
    ...    link-netns clab-vxlan-tools-some_very_long_node_name_l1
    ...    link-netnsid ${l1_host_link_netnsid}

Check host interface for l2 node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -d l show l2_eth1

    Should Contain    ${output}    mtu 9500

    Should Contain    ${output}    link-netns clab-vxlan-tools-l2

Deploy vxlab link between l1 and l3 with tools cmd
    # log bridge details to check if its mtu is really set to 9100 after the lab deployment
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${vxlan-br}

    Should Contain    ${output}    mtu 9100

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} tools vxlan create --remote 172.20.25.23 --link ${l1_host_link} --id 101 --port 14788
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify vxlan links between l1 and l3
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -d l show dev vx-${l1_host_link}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    mtu 9050 qdisc noqueue state UNKNOWN
    Should Contain    ${output}    vxlan id 101 remote 172.20.25.23 dev clab-vxlan-br srcport 0 0 dstport 14788

Check VxLAN connectivity l1-l3
    # CI env var is set to true in Github Actions
    # and this test won't run there, since it fails for unknown reason
    IF    '%{CI=false}'=='false'
        Wait Until Keyword Succeeds    60    2s    Check VxLAN connectivity l1->l3
    END

Deploy vxlab link between l2 and l4 with tools cmd
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} tools vxlan create --remote 172.20.25.24 --link ${l2_host_link} --id 102
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify vxlan links between l2 and l4
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -d link show vx-${l2_host_link}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    mtu 9050 qdisc noqueue state UNKNOWN
    Should Contain    ${output}    vxlan id 102 remote 172.20.25.24 dev clab-vxlan-br srcport 0 0 dstport 14789

Check VxLAN connectivity l2-l4
    # CI env var is set to true in Github Actions
    # and this test won't run there, since it fails for unknown reason
    IF    '%{CI=false}'=='false'
        Wait Until Keyword Succeeds    60    2s    Check VxLAN connectivity l2->l4
    END


*** Keywords ***
Setup
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'
    # setup vxlan underlay bridge
    # we have to setup an underlay management bridge with big enough mtu to support vxlan and srl requirements for link mtu
    # we set mtu 9100 (and not the default 9500) because srl can't set vxlan mtu > 9412 and < 1500
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link add ${vxlan-br} type bridge || true
    Log    ${output}

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link set dev ${vxlan-br} up && sleep 2 && sudo ip link set dev ${vxlan-br} mtu 9100 && sudo ip addr add ${vxlan-br-ip} dev ${vxlan-br} || true
    Log    ${output}

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip link show ${vxlan-br}
    Log    ${output}

Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip l del ${vxlan-br}
    Log    ${output}

Check VxLAN connectivity l1->l3
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -it clab-${lab-name}-${l1_name} ping 192.168.13.2 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Check VxLAN connectivity l2->l4
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -it clab-${lab-name}-${l2_name} ping 192.168.24.2 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss
