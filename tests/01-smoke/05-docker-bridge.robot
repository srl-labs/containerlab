*** Settings ***
Library             OperatingSystem
Library             String

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}     05-docker-bridge
${lab-file}     05-docker-bridge.clab.yml
${runtime}      docker


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure inspect outputs IP addresses
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} inspect --name ${lab-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${line} =    String.Get Line    ${output}    -2
    Log    ${line}
    @{data} =    Split String    ${line}    |
    Log    ${data}
    # verify ipv4 address
    ${ipv4} =    String.Strip String    ${data}[7]
    Should Match Regexp    ${ipv4}    ^[\\d\\.]+/\\d{1,2}$


*** Keywords ***
Setup
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'

Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}
