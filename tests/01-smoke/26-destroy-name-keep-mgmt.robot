*** Comments ***
This test suite verifies destroy by name works when --keep-mgmt-net is used
and the topology file is missing (issue requirement)

*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup

*** Variables ***
${lab-name}         26-destroy-name-keep-mgmt
${topo}             ${CURDIR}/26-test-lab.clab.yml
${mgmt-bridge}      01-26-net

*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Remove topology file
    Remove File    ${topo}
    File Should Not Exist    ${topo}

Verify lab is still running
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --name ${lab-name}
    Log    \n--> LOG: Inspect output\n${output}    console=True
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${lab-name}

Destroy lab by name with --keep-mgmt-net (topology file missing)
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy --name ${lab-name} --keep-mgmt-net
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    not found, proceeding with limited cleanup
    Should Contain    ${output}    Destroying lab

Verify lab containers are removed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --name ${lab-name}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    no containers found

Verify management network is kept
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ip l show dev ${mgmt-bridge}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0


*** Keywords ***
Setup
    # Create test topology file
    Create File    ${topo}    name: ${lab-name}
    ...    \nmgmt:
    ...    \n  bridge: ${mgmt-bridge}
    ...    \ntopology:
    ...    \n  nodes:
    ...    \n    node1:
    ...    \n      kind: linux
    ...    \n      image: alpine:3
    ...    \n      cmd: ash -c "sleep 9999"

Cleanup
    # Make sure any remaining resources are cleaned up
    Run    ${CLAB_BIN} --runtime ${runtime} destroy --name ${lab-name} --cleanup || true
    Run    sudo ip link delete ${mgmt-bridge} || true
    Remove File    ${topo}