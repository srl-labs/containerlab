*** Settings ***
Library           OperatingSystem

*** Test Cases ***
Show containerlab version
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output    sudo containerlab version
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
