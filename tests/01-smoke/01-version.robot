*** Settings ***
Library           OperatingSystem

*** Test Cases ***
Show containerlab version
    ${rc}    ${output} =    Run And Return Rc And Output    containerlab version
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
