*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-name}         mgmtnetif
${topo}             ${CURDIR}/16-mgmtnetinterface.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    ${result} =    Run Process   
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    ...    shell=True
    Log    ${result.stdout}
    Should Be Equal As Integers    ${result.rc}    0

Check host side interface is attached to mgmt bridge and up
    ${params} =   Set Variable    docker network inspect clab --format '{{ $opt := index .Options "com.docker.network.bridge.name"}}{{ $opt }}'
    ${mgmtbrname} =    Run Process    ${params}    
    ...    shell=True
    Log    ${mgmtbrname.stdout}
    ${result} =    Run Process
    ...    sudo -E ip link show dev l1eth1
    ...    shell=True
    Log    ${result.stdout}
    Should Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stdout}    state UP
    Should Contain    ${result.stdout}    master ${mgmtbrname.stdout}

*** Keywords ***
Teardown
    # destroy all labs
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -c -a

Setup
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'
