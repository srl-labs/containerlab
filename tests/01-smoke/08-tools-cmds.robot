*** Comments ***
This test suite verifies
- the operation of tools veth create command
- the operation of tools netem command


*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Teardown      Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup


*** Variables ***
${lab-name}     2-linux-nodes
${topo}         ${CURDIR}/01-linux-nodes.clab.yml


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Create new veth pair between nodes
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} tools veth create -a clab-${lab-name}-l1:eth63 -b clab-${lab-name}-l2:eth63
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check the new interface has been created
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip netns exec clab-${lab-name}-l1 ip l show dev eth63
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    eth63

Add link impairments
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} tools netem set -n clab-${lab-name}-l1 -i eth63 --delay 100ms --jitter 2ms --loss 10 --rate 1000
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    100ms
    Should Contain    ${output}    2ms
    Should Contain    ${output}    10.00%
    Should Contain    ${output}    1000

Show link impairments
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} tools netem show -n clab-${lab-name}-l1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    100ms
    Should Contain    ${output}    2ms
    Should Contain    ${output}    10.00%
    Should Contain    ${output}    1000
