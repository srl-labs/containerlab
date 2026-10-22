*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot
Resource            ../srsim.robot

Suite Teardown      Destroy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}


*** Variables ***
${lab-name}         sr06
${lab-file-name}    06-srsim-sar.clab.yml
${runtime}          docker
${gnmic_image}      ghcr.io/openconfig/gnmic:0.49.0
${gnmic_flags}      --username admin --password NokiaSros1! --values-only


*** Test Cases ***
Deploy ${lab-name} lab
    Deploy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}

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
