*** Settings ***
Library     OperatingSystem
Resource    common.robot


*** Keywords ***
Deploy SR-SIM Lab
    [Arguments]    ${topology}    ${runtime}
    Log    ${topology}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topology}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Destroy SR-SIM Lab
    [Arguments]    ${topology}    ${runtime}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topology} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
