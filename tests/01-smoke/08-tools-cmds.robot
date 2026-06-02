*** Comments ***
This test suite verifies
- the operation of tools veth create command
- the operation of tools netem command


*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup


*** Variables ***
${runtime}      docker
${lab-name}     2-linux-nodes
${topo}         ${CURDIR}/01-linux-nodes.clab.yml


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Add link impairments
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools netem set -n clab-${lab-name}-l1 -i eth3 --delay 100ms --jitter 2ms --loss 10 --rate 1000 --corruption 2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    100ms
    Should Contain    ${output}    2ms
    Should Contain    ${output}    10.00%
    Should Contain    ${output}    1000
    Should Contain    ${output}    2.00%

Show link impairments
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools netem show -n clab-${lab-name}-l1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    100ms
    Should Contain    ${output}    2ms
    Should Contain    ${output}    10.00%
    Should Contain    ${output}    1000

Show link impairments in JSON format
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools netem show -n clab-${lab-name}-l1 --format json
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # Verify that the output contains the node key
    Should Contain    ${output}    "clab-${lab-name}-l1"
    # Verify the JSON keys exist in the impairment objects.
    Should Contain    ${output}    "interface"
    Should Contain    ${output}    "delay"
    Should Contain    ${output}    "jitter"
    Should Contain    ${output}    "packet_loss"
    Should Contain    ${output}    "rate"
    Should Contain    ${output}    "corruption"
    # Verify the expected values appear
    Should Contain    ${output}    "100ms"
    Should Contain    ${output}    "2ms"
    Should Contain    ${output}    10
    Should Contain    ${output}    1000
    Should Contain    ${output}    2

Reset link impairments
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools netem reset -n clab-${lab-name}-l1 -i eth3
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Reset impairments on node "clab-${lab-name}-l1", interface "eth3"

    # Show impairments again to verify they have been reset.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools netem show -n clab-${lab-name}-l1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # Verify that the previous impairment values are no longer present.
    Should Not Contain    ${output}    100ms
    Should Not Contain    ${output}    2ms
    Should Not Contain    ${output}    10.00%
    Should Not Contain    ${output}    1000