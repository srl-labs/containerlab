*** Settings ***
Library           OperatingSystem
Library           SSHLibrary
Suite Teardown    Run Keyword    Cleanup
Resource          ../common.robot

*** Variables ***
${lab-name}       03-01-two-ceos
${lab-file-name}    03-ceos01-clab.yml
${node1-name}     n1
${node2-name}     n2
${n1-mgmt-ip}
${n2-mgmt-ip}     172.20.20.22

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

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
    ${f} =    OperatingSystem.Get File    ${EXECDIR}/clab-${lab-name}/${node1-name}/flash/startup-config
    Log    ${f}
    Log    ${n1-mgmt-ip}
    Should Contain    ${f}    ${n1-mgmt-ip}

Ensure n2 mgmt IPv4 is in the config file
    ${f} =    OperatingSystem.Get File    ${EXECDIR}/clab-${lab-name}/${node2-name}/flash/startup-config
    Log    ${f}
    Should Contain    ${f}    ${n2-mgmt-ip}

Ensure n1 is reachable over ssh
    Common.Login via SSH with username and password
    ...    address=${n1-mgmt-ip}
    ...    username=admin
    ...    password=admin
    ...    try_for=30

Ensure n2 is reachable over ssh
    Common.Login via SSH with username and password
    ...    address=${n2-mgmt-ip}
    ...    username=admin
    ...    password=admin
    ...    try_for=30

*** Keywords ***
Cleanup
    Run    sudo containerlab destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}
