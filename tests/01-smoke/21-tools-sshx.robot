*** Comments ***
This test suite verifies the functionality of the SSHX terminal sharing operations:
- Attaching an SSHX container to a lab network
- Listing active SSHX containers
- Detaching an SSHX container from a lab network
- Verifying the SSHX container is properly removed

*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup

*** Variables ***
${runtime}          docker
${lab-name}         sshx-test
${topo}             ${CURDIR}/01-linux-nodes.clab.yml
${network-name}     clab
${sshx-container}   ${lab-name}

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Attach SSHX container to lab network
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx attach -n ${network-name} --name ${sshx-container}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    SSHX container ${sshx-container} started
    Should Contain    ${output}    SSHX link for collaborative terminal access:
    Should Contain    ${output}    https://sshx.io/

List SSHX containers
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx list
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${sshx-container}
    Should Contain    ${output}    ${network-name}
    Should Contain    ${output}    running

List SSHX containers in JSON format
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx list --format json
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    "${sshx-container}"
    Should Contain    ${output}    "${network-name}"
    Should Contain    ${output}    "running"
    Should Contain    ${output}    "ipv4_address"
    Should Contain    ${output}    "link"
    Should Contain    ${output}    "owner"

Detach SSHX container from lab network
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx detach -n ${network-name} --name ${sshx-container}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    SSHX container ${sshx-container} removed successfully

Verify SSHX container is removed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools sshx list
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    ${sshx-container}

    # Also verify with the runtime directly
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} ps -a | grep ${sshx-container}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0