*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-file}     stages.clab.yml
${lab-name}     stages
${runtime}      docker


*** Test Cases ***
Test filter 1
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0


*** Keywords ***
Teardown
    # destroy all labs
    Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -c -a

# Setup
#    # skipping this test suite for podman for now
#    Skip If    '${runtime}' == 'podman'
