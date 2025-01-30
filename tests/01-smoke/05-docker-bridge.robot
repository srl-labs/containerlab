*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}         05-docker-bridge
${lab-file}         05-docker-bridge.clab.yml
${runtime}          docker
${table-delimit}    │


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    Log    \n--> LOG: Deploy output\n${output}    console=True
    Should Be Equal As Integers    ${rc}    0

Ensure inspect outputs IP addresses
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --name ${lab-name}
    Log    \n--> LOG: Inspect output\n${output}    console=True
    Should Be Equal As Integers    ${rc}    0

    ${line} =    String.Get Line    ${output}    -3
    Log    \n--> LOG: Fetched line\n${line}    console=True

    @{data} =    Split String    ${line}    ${table-delimit}
    Log    \n--> LOG: Fetched data\n${data}    console=True

    # verify ipv4 address
    ${ipv4} =    String.Strip String    ${data}[4]
    Should Match Regexp    ${ipv4}    ^[\\d\\.]+$


*** Keywords ***
Setup
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'

Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}
