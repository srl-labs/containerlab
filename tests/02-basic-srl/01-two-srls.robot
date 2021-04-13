*** Settings ***
Library           OperatingSystem
Suite Teardown    Run Keyword    Cleanup

*** Variables ***
${lab-name}       02-01-two-srls

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab deploy -t ${CURDIR}/02-srl02.clab.yml
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait 5 seconds
    Sleep    5s

Verify links in node srl1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec clab-${lab-name}-srl1 ip link show e1-1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in node srl2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec clab-${lab-name}-srl2 ip link show e1-1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

*** Keywords ***
Cleanup
    Run    sudo containerlab destroy -t ${CURDIR}/02-srl02.clab.yml --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}
