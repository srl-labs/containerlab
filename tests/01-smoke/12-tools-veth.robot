*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Setup         Run Keyword    Setup
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-file}                 12-tools-veth.clab.yaml
${lab-name}                 dual-node
${runtime}                  docker
${bridge-name}              clabtestbr
${bridge-n1-iface}          n1eth1
${host-n1-iface}            n1hosteth1
# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test
${runtime-cli-exec-cmd}     sudo docker exec


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}

Define runtime exec command
    IF    "${runtime}" == "podman"
        Set Suite Variable    ${runtime-cli-exec-cmd}    sudo podman exec
    END

Verify links in node n1 pre-deploy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n1 ip link show eth0
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Deploy veth between bridge and n1
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools veth create -d -a clab-${lab-name}-n1:eth1 -b bridge:${bridge-name}:${bridge-n1-iface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node n1 post-deploy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n1 ip link show eth1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify bridge link
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip l show ${bridge-n1-iface}
    Log    ${output}
    Should Contain    ${output}    master ${bridge-name} state UP

Deploy veth between n1 and n2
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools veth create -d -a clab-${lab-name}-n1:eth2 -b clab-${lab-name}-n2:eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node n1 post-deploy 2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n1 ip link show eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in node n2 post-deploy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n2 ip link show eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Deploy veth between n2 and host
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools veth create -d -a clab-${lab-name}-n2:eth3 -b host:${host-n1-iface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify links in node n2 post-deploy 2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-n2 ip link show eth3
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Verify links in host 2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show dev ${host-n1-iface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP


*** Keywords ***
Setup
    Run    sudo ip l add dev ${bridge-name} type bridge

Teardown
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Run    sudo ip l del dev ${bridge-name}
