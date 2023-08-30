*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}     vxlan
${lab-file}     01-vxlan.clab.yml
${runtime}      docker


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file} -d
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check VxLAN connectivity srl-linux
    Wait Until Keyword Succeeds    15    2s    Check VxLAN connectivity srl-linux

Check VxLAN connectivity linux-srl
    Wait Until Keyword Succeeds    15    2s    Check VxLAN connectivity linux-srl

*** Keywords ***
Check VxLAN connectivity srl-linux
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -it clab-vxlan-s1 ip netns exec srbase-mgmt ping 192.168.67.1 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Check VxLAN connectivity linux-srl
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec vxlep ping 192.168.67.2 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss
     
Setup
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'
    # setup vxlan termination namespace
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CURDIR}/01-host-setup.sh
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0    

Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}
