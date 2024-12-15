*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         iol
${lab-file-name}    iol.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait 45s for nodes to boot
    Sleep    45s

Verify links in node router1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sshpass -p "admin" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-router1 sh ip int br | head -5
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    172.20.20.
    Should Contain    ${output}    up

Verify links in node switch
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sshpass -p "admin" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-switch sh ip int br | head -5 | tail -1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    172.20.20.
    Should Contain    ${output}    up

Verify parital startup configuration on router2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sshpass -p "admin" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-router2 show running-config interface Loopback0
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    PARTIAL_CFG

Verify full startup configuration on router3
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sshpass -p "admin" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-router3 "sh run | inc hostname"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    FULL_STARTUP_CFG-router3


*** Keywords ***
Cleanup
    Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
