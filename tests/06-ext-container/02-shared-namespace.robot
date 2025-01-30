*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         06-shared-namespace
${lab-file-name-1}    02-shared-namespace-ext.clab.yaml
${lab-file-name-2}    02-shared-namespace.clab.yaml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name}-ext lab
    Log    ${CURDIR}
    ${output} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -d -t ${CURDIR}/${lab-file-name-1}
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${output} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -d -t ${CURDIR}/${lab-file-name-2}
    ...    shell=True
    ...    timeout=30s
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

Verify ip on ext-node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec --label clab-node-name\=ext-node --cmd "ip address show dev d1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    128.66.0.1/32

Verify links in node0
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec --label clab-node-name\=node0 --cmd "ip link show dev net0"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify ext-node defined interface is present for node1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec --label clab-node-name\=node1 --cmd "ip address show dev d1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    128.66.0.1/32

Verify topo defined link in node1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec --label clab-node-name\=node1 --cmd "ip link show dev net0"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP


*** Keywords ***
Setup
    Cleanup

Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name-1} --cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name-2} --cleanup
