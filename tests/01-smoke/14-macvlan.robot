*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-name}         macvlan
${topo}             ${CURDIR}/macvlan.clab.yml
${runtime}          docker
# interface inside l1 node that should be macvlan
${macvlan-iface}    eth1


*** Test Cases ***
Find parent interface
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -j route get 8.8.8.8 | jq -r .[].dev
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    Set Suite Variable    ${parent}    ${output}

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip -j l show ${parent} | jq -r .[].mtu
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    Set Suite Variable    ${parent_mtu}    ${output}

Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E host_link=${parent} ${CLAB_BIN} --runtime ${runtime} deploy -c -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check macvlan interface on l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec clab-${lab-name}-l1 ip -d link show ${macvlan-iface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    Should Contain    ${output}    macvlan mode bridge
    Should Contain    ${output}    mtu ${parent_mtu}


*** Keywords ***
Teardown
    # destroy all labs
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -c -a

Setup
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'
