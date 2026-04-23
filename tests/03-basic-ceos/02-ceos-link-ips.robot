*** Settings ***
Library             OperatingSystem
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         03-01-two-ceos
${lab-file-name}    03-ceos01-clab.yml
${runtime}          docker
${n1-link-ip}       192.168.55.1
${n2-link-ip}       192.168.55.2


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait for dataplane to settle
    Sleep    10s

Verify n1 has link IPv4 configured
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=n1 --cmd "Cli -p 15 -c 'show ip interface brief'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${n1-link-ip}

Ensure n1 can ping n2 over eth1 link
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=n1 --cmd "Cli -p 15 -c 'ping ${n2-link-ip} repeat 3'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Success rate is 100 percent


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}
