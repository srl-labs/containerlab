*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Setup            Run Keyword    Setup
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-file}                 01-tolopogy.clab.yaml
${lab-name}                 dual-node
${runtime}                  docker
${bridge-name}              clabtestbr
${runtime-cli-exec-cmd}     sudo docker exec
# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}

Verify links in node n1 pre-deploy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n1 ip link show eth0
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Deploy veth between bridge and n1
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} tools veth create -d -a clab-${lab-name}-n1:n11 -b bridge:${bridge-name}:n1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node n1 post-deploy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n1 ip link show n11
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify bridge link
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    brctl show dev ${bridge-name}
    Log    ${output}
    Should Contain    ${output}    n1

Deploy veth between n1 and n2
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} tools veth create -d -a clab-${lab-name}-n1:n2 -b clab-${lab-name}-n2:n1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node n1 post-deploy 2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n1 ip link show n2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in node n2 post-deploy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n2 ip link show n1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Deploy veth between n2 and host
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${CLAB_BIN} --runtime ${runtime} tools veth create -d -a clab-${lab-name}-n2:h1 -b host:n2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node n2 post-deploy 2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n2 ip link show h1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in host 2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show dev n2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

*** Keywords ***
Setup
    Run    sudo ip l add dev ${bridge-name} type bridge

Teardown
    Run    sudo ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Run    sudo ip l del dev ${bridge-name}