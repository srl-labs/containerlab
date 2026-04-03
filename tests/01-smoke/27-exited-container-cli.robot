*** Comments ***
This test suite verifies that when a node's main process exits immediately (e.g. ip route
fails with RTNETLINK network unreachable, etc) while the topology defines a link to that
node, containerlab surfaces that container output in the CLI and does not show the
misleading /proc/0/ns/net Statfs error (issue #2284).


*** Settings ***
Library             OperatingSystem
Library             Process
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup


*** Variables ***
${lab-name}         27-exited-container-cli
${topo}             ${CURDIR}/27-exited-container-cli.clab.yml


*** Test Cases ***
Deploy ${lab-name} fails with clear CLI output
    ${res} =    Run Process    ${CLAB_BIN}    --runtime    ${runtime}    deploy    -t    ${topo}
    ...    stderr=STDOUT
    Log    ${res.stdout}
    Should Not Be Equal As Integers    ${res.rc}    0
    Should Contain    ${res.stdout}    RTNETLINK answers: Network unreachable
    Should Not Contain    ${res.stdout}    /proc/0/ns/net


*** Keywords ***
Setup
    Skip If    '${runtime}' == 'podman'
    Run Keyword And Ignore Error
    ...    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
