*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../ssh.robot
Resource            ../common.robot
Resource            ../srsim.robot

Suite Setup         Skip If    '${26_10_TESTS}'.lower() != 'true'
...                 SR-SIM 26.10 is required; enable with --variable 26_10_TESTS:true
Suite Teardown      Destroy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}


*** Variables ***
${lab-name}         srsim-prompt-password
${lab-file-name}    15-srsim-prompt-password.clab.yml
${runtime}          docker
${26_10_TESTS}      false
${address}          clab-${lab-name}-srsim
${current-password}    NokiaSros1!
${new-password}        Zebra-Falcon42!


*** Test Cases ***
Deploy ${lab-name} lab
    Deploy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}

Change admin password at login
    ${output} =    Open Connection And Login    ${address}    admin    ${current-password}
    Should Contain    ${output}    You need to change your password
    Should Contain    ${output}    Enter current password:
    SSHLibrary.Write    ${current-password}
    SSHLibrary.Read Until    Enter new password:
    SSHLibrary.Write    ${new-password}
    SSHLibrary.Read Until Regexp    Re-?enter new password:
    SSHLibrary.Write    ${new-password}
    ${output} =    SSHLibrary.Read Until    \#
    Should Not Contain    ${output}    MINOR:
    SSHLibrary.Close All Connections

Login with the new password
    Login via SSH with username and password
    ...    address=${address}
    ...    username=admin
    ...    password=${new-password}
    ...    try_for=5
    SSHLibrary.Close All Connections


*** Keywords ***
Open Connection And Login
    [Arguments]    ${address}    ${username}    ${password}
    FOR    ${i}    IN RANGE    10
        SSHLibrary.Open Connection    ${address}    timeout=30
        ${status}    ${output} =    Run Keyword And Ignore Error
        ...    SSHLibrary.Login    ${username}    ${password}
        IF    '${status}' == 'PASS'    RETURN    ${output}
        SSHLibrary.Close All Connections
        Sleep    2s
    END
    Fail    Unable to connect to ${address} via SSH
