*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         veth-stitch
${lab-file-name}    12-srsim-veth-stitch.clab.yml
${runtime}          docker
${sros-password}    NokiaSros1!
${peer-ip}          10.0.0.2


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure sros1 can ping sros2 over the veth-stitch link
    Wait Until Keyword Succeeds    3 minutes    10 seconds    Ping succeeds    sros1    ${peer-ip}

Impair the veth-stitch link with 100% loss
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools netem set -n clab-${lab-name}-sros1 -i 1/1/c1/1 --loss 100
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    100.00%

Verify ping fails while the link is impaired
    ${output} =    Ping over SSH    sros1    ${peer-ip}
    Should Contain    ${output}    100% packet loss

Reset the impairment on the veth-stitch link
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools netem reset -n clab-${lab-name}-sros1 -i 1/1/c1/1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    veth-stitch

Verify ping recovers after reset
    Wait Until Keyword Succeeds    1 minute    5 seconds    Ping succeeds    sros1    ${peer-ip}

Destroy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify the stitch interfaces are removed on destroy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show
    Log    ${output}
    Should Not Contain    ${output}    clab-s-


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup

Ping over SSH
    [Arguments]    ${node}    ${dst}
    Login via SSH with username and password
    ...    address=clab-${lab-name}-${node}
    ...    username=admin
    ...    password=${sros-password}
    ...    try_for=30
    SSHLibrary.Set Client Configuration    timeout=20 seconds
    SSHLibrary.Write    ping ${dst} count 3 timeout 1
    ${output} =    SSHLibrary.Read Until    packet loss
    SSHLibrary.Close All Connections
    Log    ${output}
    RETURN    ${output}

Ping succeeds
    [Arguments]    ${node}    ${dst}
    ${output} =    Ping over SSH    ${node}    ${dst}
    Should Contain    ${output}    0.00% packet loss
