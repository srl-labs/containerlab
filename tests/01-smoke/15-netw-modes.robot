*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Setup         Run Keyword    Teardown
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab}      ${CURDIR}/netmodes.clab.yml


*** Test Cases ***
Deploy lab
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab}
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0


*** Keywords ***
Teardown
    # destroy all labs
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -c -a
