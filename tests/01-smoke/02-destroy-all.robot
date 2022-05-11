*** Comments ***
This suite tests:
- the destroy --all operation
- the host mode networking for l3 node

*** Settings ***
Library           OperatingSystem
Library         RPA.JSON
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

Verify host mode networking for node l3
    # l3 node is launched with host mode networking
    # since it is the nginx container, by launching it in the host mode
    # we should be able to curl localhost:80
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    curl localhost:80
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Thank you for using nginx

Destroy all labs
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} destroy --all --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check all labs have been removed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} inspect --all -f json
    Log    ${output}
    ${json}=          Convert String to JSON    ${output}
    @{containers}=         Get Value From Json     ${json}            $.container_data
    ${length}         Get length          ${containers} 
    Should Be Equal As Integers     0     ${length}
