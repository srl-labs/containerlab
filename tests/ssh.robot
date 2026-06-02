*** Settings ***
Library     SSHLibrary


*** Keywords ***
Login via SSH with username and password
    [Arguments]
    ...    ${address}=${None}
    ...    ${port}=22
    ...    ${username}=${None}
    ...    ${password}=${None}
    # seconds to try and successfully login
    ...    ${try_for}=4
    ...    ${conn_timeout}=3
    FOR    ${i}    IN RANGE    ${try_for}
        SSHLibrary.Open Connection    ${address}    timeout=${conn_timeout}
        ${status}=    Run Keyword And Return Status    SSHLibrary.Login    ${username}    ${password}
        IF    ${status}    BREAK
        Sleep    1s
    END
    IF    $status!=True
        Fail    Unable to connect to ${address} via SSH in ${try_for} attempts
    END
    Log    Exited the loop.

Login via SSH with public key
    [Arguments]
    ...    ${address}=${None}
    ...    ${port}=22
    ...    ${username}=${None}
    ...    ${keyfile}=~/.ssh/id_rsa
    # seconds to try and successfully login
    ...    ${try_for}=4
    ...    ${conn_timeout}=3
    Log    ${keyfile}
    FOR    ${i}    IN RANGE    ${try_for}
        SSHLibrary.Open Connection    ${address}    timeout=${conn_timeout}
        ${status}=    Run Keyword And Return Status    SSHLibrary.Login With Public Key    ${username}    ${keyfile}
        IF    ${status}    BREAK
        Sleep    1s
    END
    IF    $status!=True
        Fail    Unable to connect to ${address} via SSH in ${try_for} attempts
    END
    Log    Exited the loop.
