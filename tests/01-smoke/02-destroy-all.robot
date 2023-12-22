*** Comments ***
This suite tests:
- the destroy --all operation
- the host mode networking for l3 node
- the ipv4-range can be set for a network


*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Teardown      Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy --all --cleanup


*** Variables ***
${runtime}      docker


*** Test Cases ***
Deploy first lab
    ${result} =    Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/01-linux-nodes.clab.yml
    ...    shell=True
    Log    ${result.stdout}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    Should Exist    %{PWD}/clab-2-linux-nodes

Deploy second lab
    ${result} =    Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/01-linux-single-node.clab.yml
    ...    cwd=/tmp    # using a different cwd to check lab resolution via container labels
    ...    shell=True
    Log    ${result.stdout}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    Should Exist    /tmp/clab-single-node

Verify host mode networking for node l3
    # l3 node is launched with host mode networking
    # since it is the nginx container, by launching it in the host mode
    # we should be able to curl localhost:80
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    curl localhost:80
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Thank you for using nginx

Verify ipv4-range is set correctly
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} inspect -t ${CURDIR}/01-linux-single-node.clab.yml
    Log    ${output}
    Should Contain    ${output}    172.20.30.9/24

Destroy all labs
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy --all --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check all labs have been removed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} inspect --all
    Log    ${output}
    Should Contain    ${output}    no containers found
    Should Not Exist    /tmp/single-node
    Should Not Exist    %{PWD}/clab-2-linux-nodes
