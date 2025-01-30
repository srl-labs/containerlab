*** Settings ***
Library     OperatingSystem
Library     SSHLibrary
Library     Process
Resource    ../common.robot


*** Variables ***
${lab-name}         01-basic-k8s-kind
${lab-file-name}    01-basic-k8s-kind.clab.yml
${runtime}          docker
${if1-name}         eth1


*** Test Cases ***
Create Bridge
    Run    sudo ip link add dev br01 type bridge
    Run    sudo ip link set dev br01 up

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${result} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name} -d
    ...    timeout=800s
    ...    shell=True
    Log    ${result.stderr}
    Log    ${result.stdout}
    Should Be Equal As Integers    ${result.rc}    0

Verify link ${if1-name} in k8s-kind node k01-control-plane
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t k01-control-plane ip address show dev ${if1-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    192.168.237.1/24

Verify link ${if1-name} in k8s-kind node k01-worker
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t k01-worker ip address show dev ${if1-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    192.168.237.2/24

Verify link ${if1-name} in k8s-kind node k02-control-plane
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t k02-control-plane ip address show dev ${if1-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    192.168.237.3/24

Verify link eth2 in k8s-kind node k01-control-plane
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t k01-control-plane ip link show dev eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify link eth2 in k8s-kind node k02-control-plane
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t k02-control-plane ip link show dev eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify ping from alpine node to k01-control-plane
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t clab-01-basic-k8s-kind-alpine ping 192.168.237.1 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify ping from alpine node to k01-worker
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t clab-01-basic-k8s-kind-alpine ping 192.168.237.2 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify ping from alpine node to k02-control-plane
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t clab-01-basic-k8s-kind-alpine ping 192.168.237.3 -c 1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify kind cluster k01 nodes are ready
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t k01-control-plane kubectl get nodes | grep Ready | wc -l
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Integers    ${output}    2

Verify kind cluster k02 nodes are ready
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker exec -t k02-control-plane kubectl get nodes | grep Ready | wc -l
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Integers    ${output}    1

Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -rf ${CURDIR}/${lab-name}
    Run    sudo ip l set dev br01 down
    Run    sudo ip l del dev br01

Verify kind nodes are gone
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E docker ps -a
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    k01-control-plane
    Should Not Contain    ${output}    k01-worker
    Should Not Contain    ${output}    k02-control-plane
    Should Not Contain    ${output}    -srl01
    Should Not Contain    ${output}    -alpine
