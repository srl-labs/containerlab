*** Comments ***
This test suite verifies
- a management network with skip-when-unused set is not created when every node
  attached to it runs with network-mode: none
- management networks in use, or without the flag, are created


*** Settings ***
Library             OperatingSystem
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${runtime}          docker
${topo}             ${CURDIR}/14-skip-when-unused.clab.yml


*** Test Cases ***
Deploy lab with a used and an unused management network
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Network Driver Should Be    clab-mgmtskip-main    bridge

Management network without nodes in use is skipped
    Network Should Not Exist    clab-mgmtskip-oob

Destroy removes the management networks
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    Network Should Not Exist    clab-mgmtskip-main
    Network Should Not Exist    clab-mgmtskip-oob


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    Multiple management networks require Docker.
    Run Keyword And Ignore Error
    ...    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup

Cleanup
    Run Keyword And Ignore Error
    ...    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
