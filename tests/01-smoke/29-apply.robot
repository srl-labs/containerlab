*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown


*** Variables ***
${lab-name}                 apply
${runtime}                  docker
${initial-topo}             29-apply-initial.clab.yml
${add-link-topo}            29-apply-add-link.clab.yml
${add-special-links-topo}   29-apply-add-special-links.clab.yml
${add-node-topo}            29-apply-add-node.clab.yml
${runtime-cli-exec-cmd}     docker exec
${recovery-timeout}         30s
${retry-interval}           2s


*** Test Cases ***
Apply initial lab
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${initial-topo}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deployed lab
    Should Contain    ${output}    apply
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Node Succeeds
    ...    l1
    ...    172.17.0.2

Dry-run reports link additions
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${add-link-topo} --dry-run
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply plan
    Should Contain    ${output}    added links
    Should Contain    ${output}    l1:eth2
    Interface Should Not Exist    l1    eth2
    Interface Should Not Exist    l2    eth2

Apply adds link between existing nodes
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${add-link-topo}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply summary
    Should Contain    ${output}    added links
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Exist    l1    eth2
    Interface Should Exist    l2    eth2

Apply deletes link between existing nodes
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${initial-topo}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted endpoints
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Not Exist    l1    eth2
    Interface Should Not Exist    l2    eth2

Dry-run reports supported non-veth link additions
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${add-special-links-topo} --dry-run
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Apply plan
    Should Contain    ${output}    added links
    Should Contain    ${output}    l1:host1
    Should Contain    ${output}    l1:mgmt1
    Should Contain    ${output}    l2:dummy1
    Interface Should Not Exist    l1    host1
    Interface Should Not Exist    l1    mgmt1
    Interface Should Not Exist    l2    dummy1
    Host Interface Should Not Exist    apphost1
    Host Interface Should Not Exist    appmgmt1

Apply adds supported non-veth links
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${add-special-links-topo}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added links
    Interface Should Exist    l1    host1
    Interface Should Exist    l1    mgmt1
    Interface Should Exist    l2    dummy1
    Host Interface Should Exist    apphost1
    Host Interface Should Exist    appmgmt1

Apply deletes supported non-veth links
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${initial-topo}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted endpoints
    Interface Should Not Exist    l1    host1
    Interface Should Not Exist    l1    mgmt1
    Interface Should Not Exist    l2    dummy1
    Host Interface Should Not Exist    apphost1
    Host Interface Should Not Exist    appmgmt1

Apply adds node and link
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${add-node-topo}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    added nodes
    Should Contain    ${output}    l3
    Node Should Be Running    l3
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Exist    l1    eth3
    Interface Should Exist    l3    eth1
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Node Succeeds
    ...    l1
    ...    172.17.0.2
    Wait Until Keyword Succeeds
    ...    ${recovery-timeout}
    ...    ${retry-interval}
    ...    Ping From Node Succeeds
    ...    l1
    ...    172.17.1.2

Apply deletes node and link
    ${rc}    ${output} =    Run Clab Command    apply -t ${CURDIR}/${initial-topo}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    deleted nodes
    Should Contain    ${output}    l3
    Node Should Not Exist    l3
    Interface Should Exist    l1    eth1
    Interface Should Exist    l2    eth1
    Interface Should Not Exist    l1    eth3


*** Keywords ***
Setup
    Run Clab Command    destroy --name ${lab-name} --cleanup

Teardown
    Run Clab Command    destroy --name ${lab-name} --cleanup

Run Clab Command
    [Arguments]    ${args}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ${args}
    ...    stderr=STDOUT
    Log    ${output}
    RETURN    ${rc}    ${output}

Interface Should Exist
    [Arguments]    ${node}    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ip link show ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${interface}

Interface Should Not Exist
    [Arguments]    ${node}    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ip link show ${interface}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

Host Interface Should Exist
    [Arguments]    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show ${interface}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${interface}

Host Interface Should Not Exist
    [Arguments]    ${interface}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show ${interface}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

Node Should Be Running
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.State.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    (?im)^running\\s*$

Node Should Not Exist
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime} inspect -f '{{.State.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0

Ping From Node Succeeds
    [Arguments]    ${node}    ${destination}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-${node} ping -c 1 -W 1 ${destination}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
