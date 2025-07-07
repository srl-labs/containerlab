*** Settings ***
Library             OperatingSystem
Library             String
Library             Collections
Library             Process
Resource            ../common.robot
Library             Collections

Suite Setup         Setup
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-file}                     24-bridge-in-ns.clab.yaml
${lab-name}                     bridges-in-ns
${runtime}                      docker


*** Test Cases ***
Deploy ${lab-name} lab
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

    Should Be Equal As Integers    ${output.rc}    0

    Set Suite Variable    ${deploylog}    ${output}

Ensure bridge br01 exists in bp1 container and is up
    [Documentation]    Ensure br01 is exists in container bp1 
    ${result} =    Process.Run Process
    ...    sudo -E docker exec clab-${lab-name}-bp1 ip link show dev br01
    ...    shell=True

    Log    ${result.stdout}
    Log    ${result.stderr}

    Should Be Equal As Integers    ${result.rc}    0

    Should Contain    ${result.stdout}    UP,LOWER_UP

Ensure veth of client is attached to br01
    [Documentation]    Ensure client veth inteface c1eth1 is attached to the bridge br01
    ${result} =    Process.Run Process
    ...    sudo -E docker exec clab-${lab-name}-bp1 ip link show dev c1eth1
    ...    shell=True

    Log    ${result.stdout}
    Log    ${result.stderr}

    Should Be Equal As Integers    ${result.rc}    0

    Should Contain    ${result.stdout}    state UP
    Should Contain    ${result.stdout}    master br01

Ensure communication between client c1 and bridge br01 in container bp1
    [Documentation]    Ensure communication is possible from client to bp1.
    ${result} =    Process.Run Process
    ...    sudo -E docker exec clab-${lab-name}-c1 ping 192.168.0.1 -c 1
    ...    shell=True
    
    Log    ${result.stdout}
    Log    ${result.stderr}

    Should Be Equal As Integers    ${result.rc}    0

    Should Contain    ${result.stdout}    0% packet loss


*** Keywords ***
Teardown
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -c -t ${CURDIR}/${lab-file}

Setup
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -c -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'