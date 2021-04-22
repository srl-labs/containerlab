*** Keywords ***
Login via SSH with username and password
    [Arguments]
    ...    ${address}=${None}
    ...    ${port}=22
    ...    ${username}=${None}
    ...    ${password}=${None}
    ...    ${prompt}=${None}
    # seconds to try and succesfully login
    ...    ${try_for}=4
    ...    ${conn_timeout}=3
    ...    ${newline}=LF
    FOR    ${i}    IN RANGE    ${try_for}
        SSHLibrary.Open Connection    ${n1-mgmt-ip}    timeout=30s
        ${status}=    Run Keyword And Return Status    SSHLibrary.Login    admin    admin
        Exit For Loop If    ${status}
        Sleep    1s
    END
    Run Keyword If    $status!=True    Fail    Unable to connect to ${address} via SSH in ${try_for} seconds
    Log    Exited the loop.
