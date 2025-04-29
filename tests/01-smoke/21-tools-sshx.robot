*** Comments ***
This test suite verifies the functionality of the SSHX terminal sharing operations:
- Attaching an SSHX container using lab name (-l) parameter
- Attaching an SSHX container using topology file (-t) parameter
- Testing read-only access with --enable-readers
- Testing reattach functionality
- Listing active SSHX containers
- Detaching an SSHX container from a lab network

*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup

*** Variables ***
${runtime}          docker
${lab_name}         2-linux-nodes
${topo}             ${CURDIR}/01-linux-nodes.clab.yml
${sshx_container}   clab-${lab_name}-sshx

*** Test Cases ***
Deploy Test Lab
    [Documentation]    Deploy the test lab for SSHX tests
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Attach SSHX Using Lab Name Parameter
    [Documentation]    Test attaching SSHX container using the -l (lab name) parameter
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx attach -l ${lab_name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    SSHX container ${sshx_container} started
    Should Contain    ${output}    SSHX successfully started
    Should Contain    ${output}    https://sshx.io/

List SSHX Containers
    [Documentation]    Test listing SSHX containers
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx list
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${sshx_container}
    Should Contain    ${output}    running

List SSHX Containers JSON Format
    [Documentation]    Test listing SSHX containers in JSON format
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx list --format json
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    "${sshx_container}"
    Should Contain    ${output}    "running"

Detach SSHX Using Lab Name Parameter
    [Documentation]    Test detaching SSHX container using the -l (lab name) parameter
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx detach -l ${lab_name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    SSHX container ${sshx_container} removed successfully

    # Verify container is removed
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${runtime} ps -a | grep ${sshx_container} || true
    Log    ${output}
    Should Not Contain    ${output}    ${sshx_container}

Attach SSHX Using Topology File Parameter
    [Documentation]    Test attaching SSHX container using the -t (topology file) parameter
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx attach -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    SSHX container ${sshx_container} started
    Should Contain    ${output}    SSHX successfully started
    Should Contain    ${output}    https://sshx.io/

    # Clean up this container before the next test
    ${clean_rc}=    Run And Return Rc
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx detach -l ${lab_name}
    Log    Cleanup return code: ${clean_rc}
    Sleep    2s

Attach SSHX With Read-Only Access
    [Documentation]    Test attaching SSHX with the --enable-readers flag
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx attach -l ${lab_name} --enable-readers
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    SSHX container ${sshx_container} started
    Should Contain    ${output}    SSHX successfully started
    Should Contain    ${output}    https://sshx.io/
    Should Contain    ${output}    Read-only access link:

    # Clean up this container before the next test
    ${clean_rc}=    Run And Return Rc
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx detach -l ${lab_name}
    Log    Cleanup return code: ${clean_rc}
    Sleep    2s

Test SSHX Reattach Functionality
    [Documentation]    Test reattaching SSHX container (detach+attach)
    # First attach an SSHX container
    ${rc1}    ${output1}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx attach -l ${lab_name}
    Log    ${output1}
    Should Be Equal As Integers    ${rc1}    0
    Should Contain    ${output1}    SSHX successfully started

    # Sleep to ensure container is fully operational
    Sleep    3s

    # Now reattach the container
    ${rc2}    ${output2}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx reattach -l ${lab_name}
    Log    ${output2}
    Should Be Equal As Integers    ${rc2}    0
    Should Contain    ${output2}    SSHX successfully reattached

    # Clean up this container before the next test
    ${clean_rc}=    Run And Return Rc
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx detach -l ${lab_name}
    Log    Cleanup return code: ${clean_rc}
    Sleep    2s

Verify SSHX Container List Is Empty
    [Documentation]    Test that no SSHX containers are listed after detaching
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx list
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    ${sshx_container}