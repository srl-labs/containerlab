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
${runtime}      docker
${lab-name}     2-linux-nodes
${topo}         ${CURDIR}/01-linux-nodes.clab.yml


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Add link impairments
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} tools netem set -n clab-${lab-name}-l1 -i eth3 --delay 100ms --jitter 2ms --loss 10 --rate 1000
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
