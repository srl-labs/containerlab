*** Settings ***
Library             Process
Resource            ../common.robot

Suite Teardown      Cleanup


*** Variables ***
${lab-file}                 node-filter.clab.yml
${lab-name}                 node-filter
${runtime}                  docker
# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test
${runtime-cli-exec-cmd}     sudo docker exec


*** Test Cases ***
Test filter 1
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file} --node-filter node1,node2,node4
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Not Contain    ${output.stdout}    node3

    # check that node1 contains only two interfaces eth1 and eth3 and doesn't contain eth2
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label clab-node-name\=node1 --cmd 'ip link'
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Contain    ${output.stderr}    eth1
    Should Contain    ${output.stderr}    eth3
    Should Not Contain    ${output.stderr}    eth2

    Cleanup

Test filter 2
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file} --node-filter node1
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that only node 1 is present
    Should Not Contain    ${output.stdout}    node2
    Should Not Contain    ${output.stdout}    node3
    Should Not Contain    ${output.stdout}    node4
    Should Contain    ${output.stdout}    node1

    # check that node1 contains no interfaces besides management
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label clab-node-name\=node1 --cmd 'ip link'
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Contain    ${output.stderr}    eth0
    Should Not Contain    ${output.stderr}    eth1
    Should Not Contain    ${output.stderr}    eth2
    Should Not Contain    ${output.stderr}    eth3


*** Keywords ***
Cleanup
    Process.Run Process    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
...    shell=True
