*** Keywords ***
Login via SSH with username and password
    [Arguments]
    ...    ${address}=${None}
    ...    ${port}=22
    ...    ${username}=${None}
    ...    ${password}=${None}
    # seconds to try and succesfully login
    ...    ${try_for}=4
    ...    ${conn_timeout}=3
    FOR    ${i}    IN RANGE    ${try_for}
        SSHLibrary.Open Connection    ${address}    timeout=${conn_timeout}
        ${status}=    Run Keyword And Return Status    SSHLibrary.Login    ${username}    ${password}
        Exit For Loop If    ${status}
        Sleep    1s
    END
    Run Keyword If    $status!=True    Fail    Unable to connect to ${address} via SSH in ${try_for} attempts
    Log    Exited the loop.

Login via SSH with public key
    [Arguments]
    ...    ${address}=${None}
    ...    ${port}=22
    ...    ${username}=${None}
    ...    ${keyfile}=~/.ssh/id_rsa
    # seconds to try and succesfully login
    ...    ${try_for}=4
    ...    ${conn_timeout}=3
    Log    ${keyfile}
    FOR    ${i}    IN RANGE    ${try_for}
        SSHLibrary.Open Connection    ${address}    timeout=${conn_timeout}
        ${status}=    Run Keyword And Return Status    SSHLibrary.Login With Public Key    ${username}    ${keyfile}
        Exit For Loop If    ${status}
        Sleep    1s
    END
    Run Keyword If    $status!=True    Fail    Unable to connect to ${address} via SSH in ${try_for} attempts
    Log    Exited the loop.
