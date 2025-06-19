*** Settings ***
Library             OperatingSystem
Library             SSHLibrary
Resource            ../common.robot
Resource            ../ssh.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         12-01-two-vyos
${lab-file-name}    12-vyos01-clab.yml
${node1-name}       n1
${node2-name}       n2
${n1-mgmt-ip}       ${EMPTY}
${n2-mgmt-ip}       172.20.20.22
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    TRY
        Should Be Equal As Integers    ${rc}    0
    EXCEPT
        Log    "Unable to deploy lab"
        Run Keyword    Fatal Error
    END

Get nodes mgmt IPs
    ${rc}    ${n1-mgmt-ip} =    Run And Return Rc And Output
    ...    sudo docker inspect clab-${lab-name}-${node1-name} -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}'
    Should Be Equal As Integers    ${rc}    0
    Set Suite Variable    ${n1-mgmt-ip}
    ${rc}    ${inspected-n2-mgmt-ip} =    Run And Return Rc And Output
    ...    sudo docker inspect clab-${lab-name}-${node2-name} -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}'
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${inspected-n2-mgmt-ip}    ${n2-mgmt-ip}

Ensure n1 mgmt IPv4 is in the config file
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/${node1-name}/config/config.boot
    Log    ${f}
    Log    ${n1-mgmt-ip}
    Should Contain    ${f}    ${n1-mgmt-ip}

Ensure n2 mgmt IPv4 is in the config file
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/${node2-name}/config/config.boot
    Log    ${f}
    Should Contain    ${f}    ${n2-mgmt-ip}

Ensure n1 is reachable over ssh
    Login via SSH with username and password
    ...    address=${n1-mgmt-ip}
    ...    username=admin
    ...    password=admin
    ...    try_for=120

Ensure n2 is reachable over ssh
    Login via SSH with username and password
    ...    address=${n2-mgmt-ip}
    ...    username=admin
    ...    password=admin
    ...    try_for=120


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}
