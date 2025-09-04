*** Settings ***
Library             OperatingSystem
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         link-vars-ip
${lab-file-name}    25-link-vars-ip.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait 10s to ensure everything is up
    Sleep    10s

Verify addresses configured on n1 e1-1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-n1 sr_cli 'info flat / interface ethernet-1/1 subinterface 0'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    address 192.168.0.1/24
    Should Contain    ${output}    address 2001:db8:abc1::1/64

Verify addresses configured on n2 e1-1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-n2 sr_cli 'info flat / interface ethernet-1/1 subinterface 0'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    address 192.168.0.2/24
    Should Contain    ${output}    address 2001:db8:abc1::2/64

Verify addresses configured on n1 e1-2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-n1 sr_cli 'info flat / interface ethernet-1/2 subinterface 0'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    address 192.168.2.1/24
    Should Contain    ${output}    address 2001:db8:abc2::1/64

Verify addresses configured on n2 e1-2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-n2 sr_cli 'info flat / interface ethernet-1/2 subinterface 0'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    address 192.168.2.2/24
    Should Contain    ${output}    address 2001:db8:abc2::2/64

Verify addresses configured on n1 e1-3
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-n1 sr_cli 'info flat / interface ethernet-1/3 subinterface 0'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    address 192.168.3.1/24
    Should Contain    ${output}    address 2001:db8:abc3::1/64

Verify addresses configured on n2 e1-3
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} exec clab-${lab-name}-n2 sr_cli 'info flat / interface ethernet-1/3 subinterface 0'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    address 192.168.3.2/24
    Should Contain    ${output}    address 2001:db8:abc3::2/64


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
