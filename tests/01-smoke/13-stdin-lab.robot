*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-url}      https://gist.githubusercontent.com/hellt/9baa28d7e3cb8290ade1e1be38a8d12b/raw/03067e242d44c9bbe38afa81131e46bab1fa0c42/test.clab.yml


*** Test Cases ***
Deploy remote lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo curl -s ${lab-url} | ${CLAB_BIN} --runtime ${runtime} deploy -c -t -
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure inspect works
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --all
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0


*** Keywords ***
Teardown
    # destroy all labs
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -c -a
