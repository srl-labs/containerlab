*** Settings ***
Library           OperatingSystem
Suite Teardown    Run Keyword    Cleanup

*** Variables ***
${lab-name}       02-01-two-srls
${lab-file-name}    02-srl02.clab.yml
${runtime}        docker

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node srl1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "ip link show e1-1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in node srl2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl2 --cmd "ip link show e1-1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify e1-1 interface have been admin enabled on srl1
    [Documentation]
    ...    This test cases ensures that e1-1 interface referenced in links section
    ...    has been automatically admin enabled
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl1 --cmd "sr_cli 'show interface ethernet-1/1'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ethernet-1/1 is up

Verify srl2 accepted user-provided CLI config
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=srl2 --cmd "sr_cli 'info /system information location'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    test123

Verify saving config
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} save -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    ERRO

*** Keywords ***
Cleanup
    Run    sudo containerlab destroy -t ${CURDIR}/${lab-file-name} --cleanup
