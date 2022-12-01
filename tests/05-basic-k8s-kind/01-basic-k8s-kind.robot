*** Settings ***
Library           OperatingSystem
Library           SSHLibrary
Suite Teardown    Run Keyword    Cleanup
Resource          ../common.robot

*** Variables ***
${lab-name}       01-basic-k8s-kind
${lab-file-name}    01-basic-k8s-kind.clab.yml
${runtime}        docker
${if1-name}     eth1

*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify link ${if1-name} in k8s-kind node k01-control-plane
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec -t k01-control-plane ip address show ${if1-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    192.168.237.1/24

Verify link ${if1-name} in k8s-kind node k01-worker
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec -t k01-worker ip address show ${if1-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    192.168.237.2/24

Verify link ${if1-name} in k8s-kind node k02-control-plane
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker exec -t k02-control-plane ip address show ${if1-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    192.168.237.3/24

Cleanup
    Run    sudo containerlab --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}

Verify kind nodes are gone
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker ps -a
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output} k01-control-plane
    Should Not Contain    ${output} k01-worker
    Should Not Contain    ${output} k02-control-plane
    Should Not Contain    ${output} srl01