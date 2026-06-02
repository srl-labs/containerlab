*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}         srl-bgp
${lab-file-name}    03-srl-bgp.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify e1-1 interface have been admin enabled on srl1
    [Documentation]
    ...    This test cases ensures that e1-1 interface referenced in links section
    ...    has been automatically admin enabled
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "sr_cli 'show interface ethernet-1/1'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ethernet-1/1 is up

Ensure srl1 can ping srl2 over ethernet-1/1 interface
    Sleep    5s    give some time for networking stack to settle
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "ip netns exec srbase-default ping 192.168.0.1 -c2 -w 3"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Check BGP session is Established
    Wait Until Keyword Succeeds    5 min    5 sec    Check BGP session is Established

Check BGP session received routes count
    Wait Until Keyword Succeeds    1 min    2 sec    Check BGP session received routes count

Check BGP session sent routes count
    Wait Until Keyword Succeeds    1 min    2 sec    Check BGP session sent routes count


*** Keywords ***
Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Log    ${output}

Check BGP session is Established
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd 'sr_cli -- info from state network-instance default protocols bgp neighbor 192.168.0.1 session-state'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    established

Check BGP session received routes count
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd 'sr_cli -- info from state network-instance default protocols bgp neighbor 192.168.0.1 afi-safi ipv4-unicast received-routes'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    received-routes 2

Check BGP session sent routes count
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd 'sr_cli -- info from state network-instance default protocols bgp neighbor 192.168.0.1 afi-safi ipv4-unicast sent-routes'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    sent-routes 2

Setup
    # skipping this test suite with podman runtime
    # since two srl nodes are very slow to form bgp session
    Skip If    '${runtime}' == 'podman'
