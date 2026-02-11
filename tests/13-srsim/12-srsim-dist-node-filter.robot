*** Settings ***
Library             Process
Library             OperatingSystem
Resource            ../common.robot

Suite Teardown      Cleanup


*** Variables ***
${lab-file}         ${EXECDIR}/lab-examples/sr-sim/test-cpm-destroy.clab.yaml
${lab-name}         cpm-destroy-test
${runtime}          docker


*** Test Cases ***
Deploy full distributed lab
    [Documentation]    Deploy the manual distributed lab with 3 containers.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file}
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # All 3 containers should be running
    FOR    ${suffix}    IN    a    b    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} not running
        Should Contain    ${out}    true
    END

Destroy CPM-A auto-expands to include dependents
    [Documentation]
    ...    Destroy with --node-filter srsim10-a. The filter should auto-expand to include
    ...    srsim10-b and srsim10-1 because they share srsim10-a's network namespace.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --node-filter srsim10-a
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Verify auto-expansion happened
    Should Contain    ${output.stderr}    Auto-including node

    # All 3 containers should be gone
    FOR    ${suffix}    IN    a    b    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Not Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} still exists
    END

Redeploy and destroy line card only
    [Documentation]    Deploy the full lab, then destroy only srsim10-1. CPMs should remain.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file} --reconfigure
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Destroy only the line card
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --node-filter srsim10-1
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Line card should be gone
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect clab-${lab-name}-srsim10-1 2>&1
    Should Not Be Equal As Integers    ${rc}    0    msg=srsim10-1 still exists

    # Both CPMs should still be running
    FOR    ${suffix}    IN    a    b
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} not running
        Should Contain    ${out}    true
    END

    # Lab dir preserved
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    test -d ${EXECDIR}/lab-examples/sr-sim/clab-${lab-name}
    Should Be Equal As Integers    ${rc}    0

Redeploy line card with node-filter
    [Documentation]    Redeploy only srsim10-1. CPMs should not be recreated.
    ${rc}    ${cpm_id_before} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.Id}}' clab-${lab-name}-srsim10-a 2>&1
    Should Be Equal As Integers    ${rc}    0

    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file} --node-filter srsim10-1
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Line card should be running again
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-1 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${out}    true

    # srsim10-1 should share network namespace of srsim10-a
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.HostConfig.NetworkMode}}' clab-${lab-name}-srsim10-1 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${out}    container:clab-${lab-name}-srsim10-a

    # CPM-A should be the same container (not recreated)
    ${rc}    ${cpm_id_after} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.Id}}' clab-${lab-name}-srsim10-a 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal    ${cpm_id_before}    ${cpm_id_after}

Destroy CPM-B does not auto-expand
    [Documentation]
    ...    Destroy with --node-filter srsim10-b. No other node depends on srsim10-b's namespace,
    ...    so the filter should NOT expand. Only srsim10-b is destroyed.
    # First make sure all 3 are running
    FOR    ${suffix}    IN    a    b    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} not running
        Should Contain    ${out}    true
    END

    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --node-filter srsim10-b
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Auto-expansion should NOT have happened
    Should Not Contain    ${output.stderr}    Auto-including node

    # srsim10-b should be gone
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect clab-${lab-name}-srsim10-b 2>&1
    Should Not Be Equal As Integers    ${rc}    0    msg=srsim10-b still exists

    # srsim10-a and srsim10-1 should still be running
    FOR    ${suffix}    IN    a    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} not running
        Should Contain    ${out}    true
    END

    # Redeploy srsim10-b for the next test
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file} --node-filter srsim10-b
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

Reconfigure with node-filter srsim10-a
    [Documentation]    Reconfigure srsim10-a. Auto-expands to all 3 containers.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file} --reconfigure --node-filter srsim10-a
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Verify auto-expansion happened
    Should Contain    ${output.stderr}    Auto-including node

    # All 3 containers should be running (freshly redeployed)
    FOR    ${suffix}    IN    a    b    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} not running
        Should Contain    ${out}    true
    END


*** Keywords ***
Cleanup
    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --cleanup
    ...    shell=True
