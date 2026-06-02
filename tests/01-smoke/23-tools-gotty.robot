*** Comments ***
This test suite verifies the functionality of the GoTTY web terminal operations:
- Attaching a GoTTY container using lab name (-l) parameter
- Attaching a GoTTY container using topology file (-t) parameter
- Testing reattach functionality
- Listing active GoTTY containers
- Detaching a GoTTY container from a lab network

*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup

*** Variables ***
${runtime}          docker
${lab_name}         2-linux-nodes
${topo}             ${CURDIR}/01-linux-nodes.clab.yml
${gotty_container}   clab-${lab_name}-gotty

*** Test Cases ***
Deploy Test Lab
    [Documentation]    Deploy the test lab for GoTTY tests
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Attach GoTTY Using Lab Name Parameter
    [Documentation]    Test attaching GoTTY container using the -l (lab name) parameter
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty attach -l ${lab_name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    GoTTY container ${gotty_container} started
    Should Contain    ${output}    GoTTY web terminal successfully started

List GoTTY Containers
    [Documentation]    Test listing GoTTY containers
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty list
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${gotty_container}
    Should Contain    ${output}    running

List GoTTY Containers JSON Format
    [Documentation]    Test listing GoTTY containers in JSON format
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty list --format json
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    "${gotty_container}"
    Should Contain    ${output}    "running"

Detach GoTTY Using Lab Name Parameter
    [Documentation]    Test detaching GoTTY container using the -l (lab name) parameter
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty detach -l ${lab_name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    GoTTY container ${gotty_container} removed successfully

    # Verify container is removed
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${runtime} ps -a | grep ${gotty_container} || true
    Log    ${output}
    Should Not Contain    ${output}    ${gotty_container}

Attach GoTTY Using Topology File Parameter
    [Documentation]    Test attaching GoTTY container using the -t (topology file) parameter
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty attach -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    GoTTY container ${gotty_container} started
    Should Contain    ${output}    GoTTY web terminal successfully started

    # Clean up this container before the next test
    ${clean_rc}=    Run And Return Rc
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty detach -l ${lab_name}
    Log    Cleanup return code: ${clean_rc}
    Sleep    2s

# No read-only functionality for GoTTY; skip analogous test

    # Clean up this container before the next test
    ${clean_rc}=    Run And Return Rc
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty detach -l ${lab_name}
    Log    Cleanup return code: ${clean_rc}
    Sleep    2s

Test GoTTY Reattach Functionality
    [Documentation]    Test reattaching GoTTY container (detach+attach)
    # First attach a GoTTY container
    ${rc1}    ${output1}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty attach -l ${lab_name}
    Log    ${output1}
    Should Be Equal As Integers    ${rc1}    0
    Should Contain    ${output1}    GoTTY web terminal successfully started

    # Sleep to ensure container is fully operational
    Sleep    3s

    # Now reattach the container
    ${rc2}    ${output2}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty reattach -l ${lab_name}
    Log    ${output2}
    Should Be Equal As Integers    ${rc2}    0
    Should Contain    ${output2}    GoTTY web terminal successfully reattached

    # Clean up this container before the next test
    ${clean_rc}=    Run And Return Rc
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty detach -l ${lab_name}
    Log    Cleanup return code: ${clean_rc}
    Sleep    2s

Verify GoTTY Container List Is Empty
    [Documentation]    Test that no GoTTY containers are listed after detaching
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools gotty list
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    ${gotty_container}
