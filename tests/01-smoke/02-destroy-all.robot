*** Settings ***
Library           OperatingSystem
Suite Teardown    Run    sudo containerlab --runtime ${runtime} destroy --all --cleanup

*** Variables ***
${runtime}        docker

*** Test Cases ***
Deploy first lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} deploy -t ${CURDIR}/01-linux-nodes.clab.yml
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Deploy second lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} deploy -t ${CURDIR}/01-linux-single-node.clab.yml
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Destroy all labs
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} destroy --all --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check all labs have been removed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} inspect --all
    Log    ${output}
    Should Contain    ${output}    no containers found
