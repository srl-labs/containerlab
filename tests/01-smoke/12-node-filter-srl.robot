*** Settings ***
Library             Process
Library             OperatingSystem
Resource            ../common.robot

Suite Teardown      Cleanup


*** Variables ***
${lab-file}                 ${EXECDIR}/lab-examples/srl02/srl02.clab.yml
${lab-name}                 srl02
${runtime}                  docker
${runtime-cli-exec-cmd}     sudo docker exec


*** Test Cases ***
Deploy full lab
    [Documentation]    Deploy the full srl02 lab as a baseline
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file}
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stdout}    srl1
    Should Contain    ${output.stdout}    srl2

Destroy with node-filter srl1
    [Documentation]    Destroy only srl1. Expect srl1 removed, srl2 still running, lab dir preserved.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --node-filter srl1
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # srl1 container should be gone
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect clab-${lab-name}-srl1 2>&1
    Should Not Be Equal As Integers    ${rc}    0

    # srl2 container should still be running
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srl2 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${out}    true

    # Lab directory should be preserved
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    test -d ${EXECDIR}/lab-examples/srl02/clab-${lab-name}
    Should Be Equal As Integers    ${rc}    0

    # srl1's link endpoint (e1-1) should be cleared on srl2
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${lab-file} --label clab-node-name\=srl2 --cmd 'ip link show e1-1'
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Not Be Equal As Integers    ${output.rc}    0

Deploy with node-filter srl1
    [Documentation]    Redeploy srl1 into the existing lab. Expect srl1 created and links reconnected.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file} --node-filter srl1
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stdout}    srl1

    # Both containers should be running
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srl1 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${out}    true

    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srl2 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${out}    true

    # Link e1-1 should be reconnected on both nodes
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${lab-file} --label clab-node-name\=srl1 --cmd 'ip link show e1-1'
    ...    shell=True
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stderr}    e1-1

    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${lab-file} --label clab-node-name\=srl2 --cmd 'ip link show e1-1'
    ...    shell=True
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stderr}    e1-1

Reconfigure with node-filter srl1
    [Documentation]    Reconfigure srl1 only. srl1 destroyed and redeployed, srl2 untouched.
    # Capture srl2 container ID before reconfigure to verify it wasn't recreated
    ${rc}    ${srl2_id_before} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.Id}}' clab-${lab-name}-srl2 2>&1
    Should Be Equal As Integers    ${rc}    0

    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file} --node-filter srl1 --reconfigure
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # srl1 should be running
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srl1 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${out}    true

    # srl2 should be the same container (not recreated)
    ${rc}    ${srl2_id_after} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.Id}}' clab-${lab-name}-srl2 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal    ${srl2_id_before}    ${srl2_id_after}

    # srl1 node directory should exist (recreated)
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    test -d ${EXECDIR}/lab-examples/srl02/clab-${lab-name}/srl1
    Should Be Equal As Integers    ${rc}    0


Invalid node filter returns error
    [Documentation]    Using a non-existent node in the filter should return an error.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --node-filter nonexistent
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Not Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stderr}    not present in the topology


*** Keywords ***
Cleanup
    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --cleanup
    ...    shell=True
