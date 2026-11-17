*** Comments ***
This suite tests the link impairments (netem) defined in the topology: containerlab sets them when
the lab is deployed, sets them again when a node is started or restarted, and updates them on apply.


*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Setup         Run Clab Command    destroy --name ${lab-name} --cleanup
Suite Teardown      Run Clab Command    destroy --name ${lab-name} --cleanup


*** Variables ***
${lab-name}                 link-netem
${runtime}                  docker
${topo}                     34-link-netem.clab.yml
${changed-vars}             34-link-netem.vars.changed.yml
${invalid-vars}             34-link-netem.vars.invalid.yml
${runtime-cli-exec-cmd}     docker exec


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run Clab Command
    ...    deploy -t ${CURDIR}/${topo}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Set link impairments

Impairments of a brief link are set
    Qdisc Should Contain    l1    eth1    delay 50ms
    Qdisc Should Contain    l1    eth1    loss 1%
    Qdisc Should Contain    l2    eth1    rate 10Mbit

Impairment of an extended link is set
    Qdisc Should Contain    l2    eth2    corrupt 2%
    Qdisc Should Not Contain    l1    eth2    netem

Packets are delayed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime-cli-exec-cmd} clab-${lab-name}-l1 ping -c 3 -i 0.2 10.0.0.2
    Log    ${output}
    # a ping may be lost, so only the minimum RTT is checked
    ${min-rtt} =    Get Regexp Matches    ${output}    = ([0-9.]+)/    1
    Should Not Be Empty    ${min-rtt}
    Should Be True    ${min-rtt}[0] >= 45

Restarted node gets its impairments again
    ${rc}    ${output} =    Run Clab Command
    ...    restart -t ${CURDIR}/${topo} --node l1
    Should Be Equal As Integers    ${rc}    0
    Qdisc Should Contain    l1    eth1    delay 50ms

Started node gets its impairments again
    ${rc}    ${output} =    Run Clab Command
    ...    stop -t ${CURDIR}/${topo} --node l2
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${output} =    Run Clab Command
    ...    start -t ${CURDIR}/${topo} --node l2
    Should Be Equal As Integers    ${rc}    0
    Qdisc Should Contain    l2    eth1    rate 10Mbit
    Qdisc Should Contain    l2    eth2    corrupt 2%

Apply updates a changed impairment
    ${rc}    ${output} =    Run Clab Command
    ...    apply -t ${CURDIR}/${topo} --vars ${CURDIR}/${changed-vars}
    Should Be Equal As Integers    ${rc}    0
    Qdisc Should Contain    l1    eth1    delay 200ms

Apply rejects an invalid impairment
    ${rc}    ${output} =    Run Clab Command
    ...    apply -t ${CURDIR}/${topo} --vars ${CURDIR}/${invalid-vars}
    Should Not Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    netem delay "50" is not a duration
    Qdisc Should Contain    l1    eth1    delay 200ms


*** Keywords ***
Run Clab Command
    [Arguments]    ${args}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ${args} 2>&1
    Log    ${output}
    RETURN    ${rc}    ${output}

Get Qdisc
    [Arguments]    ${node}    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip netns exec clab-${lab-name}-${node} tc qdisc show dev ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

Qdisc Should Contain
    [Arguments]    ${node}    ${interface}    ${impairment}
    ${qdisc} =    Get Qdisc    ${node}    ${interface}
    Should Contain    ${qdisc}    ${impairment}

Qdisc Should Not Contain
    [Arguments]    ${node}    ${interface}    ${impairment}
    ${qdisc} =    Get Qdisc    ${node}    ${interface}
    Should Not Contain    ${qdisc}    ${impairment}
