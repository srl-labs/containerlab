*** Settings ***
Library           OperatingSystem
Suite Teardown    Run    sudo containerlab destroy --all --cleanup

*** Test Cases ***
Deploy first lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab deploy -t ${CURDIR}/01-linux-nodes.clab.yml
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Deploy second lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab deploy -t ${CURDIR}/01-linux-single-node.clab.yml
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Destroy all labs
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab destroy --all
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check all labs have been removed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab inspect --all
    Log    ${output}
    Should Contain    ${output}    no containers found
