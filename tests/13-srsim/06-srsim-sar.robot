*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         sr06
${lab-file-name}    06-srsim-sar.clab.yml
${runtime}          docker
${key-name}         clab-test-key
${gnmic_image}      ghcr.io/openconfig/gnmic:0.42.1
${gnmic_flags}      --username admin --password NokiaSros1! --values-only


*** Test Cases ***
Set key-path Variable
    ${key-path} =    OperatingSystem.Normalize Path    ~/.ssh/${key-name}
    Set Suite Variable    ${key-path}

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure sros is reachable over ssh
    Login via SSH with username and password
    ...    address=clab-${lab-name}-sar3
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=10


Check Config Mode on sar1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --skip-verify --address clab-${lab-name}-sar1 get --path /state/system/management-interface/configuration-oper-mode
    Log    ${output}
    Should Contain    ${output}    model-driven


Check Config Mode on sar2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} run --network host --rm ${gnmic_image} get ${gnmic_flags} --insecure --address clab-${lab-name}-sar2 get --path /state/system/management-interface/configuration-oper-mode
    Log    ${output}
    Should Contain    ${output}    model-driven

*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*
