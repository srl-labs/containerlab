*** Settings ***
Library             OperatingSystem
Resource            ../common.robot
Resource            ../ssh.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         ceos-partial-test
${lab-file-name}    partial-clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify default configuration on ceos1
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/ceos1/flash/startup-config
    Log    ${f}
    Should Contain    ${f}    hostname ceos1
    Should Contain    ${f}    username admin
    Should Contain    ${f}    management api gnmi

Verify partial startup configuration on ceos2
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/ceos2/flash/startup-config
    Log    ${f}
    Should Contain    ${f}    hostname ceos2
    Should Contain    ${f}    username admin
    Should Contain    ${f}    management api gnmi
    Should Contain    ${f}    description PARTIAL_CONFIG_TEST
    Should Contain    ${f}    interface Ethernet1

Verify full startup configuration on ceos3
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/ceos3/flash/startup-config
    Log    ${f}
    Should Contain    ${f}    hostname ceos3-full
    Should Not Contain    ${f}    management api gnmi

Destroy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/clab-${lab-name}
