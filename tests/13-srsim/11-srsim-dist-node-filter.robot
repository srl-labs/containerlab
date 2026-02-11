*** Settings ***
Library             Process
Library             OperatingSystem
Resource            ../common.robot

Suite Teardown      Cleanup


*** Variables ***
${lab-file}         ${EXECDIR}/lab-examples/sr-sim/lab-distributed.clab.yaml
${lab-name}         sros
${runtime}          docker


*** Test Cases ***
Deploy full distributed lab
    [Documentation]    Deploy the full manual distributed lab with 8 containers (4 per system).
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file}
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # All 8 containers should be running
    FOR    ${node}    IN    srsim10    srsim11
        FOR    ${suffix}    IN    a    b    1    2
            ${rc}    ${out} =    Run And Return Rc And Output
            ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-${node}-${suffix} 2>&1
            Should Be Equal As Integers    ${rc}    0    msg=${node}-${suffix} not running
            Should Contain    ${out}    true
        END
    END

Destroy line cards only
    [Documentation]    Destroy only line cards from both systems. CPMs should remain running.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --node-filter srsim10-1,srsim10-2,srsim11-1,srsim11-2
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # All 4 line card containers should be gone
    FOR    ${node}    IN    srsim10    srsim11
        FOR    ${suffix}    IN    1    2
            ${rc}    ${out} =    Run And Return Rc And Output
            ...    sudo docker inspect clab-${lab-name}-${node}-${suffix} 2>&1
            Should Not Be Equal As Integers    ${rc}    0    msg=${node}-${suffix} still exists
        END
    END

    # All 4 CPM containers should still be running
    FOR    ${node}    IN    srsim10    srsim11
        FOR    ${suffix}    IN    a    b
            ${rc}    ${out} =    Run And Return Rc And Output
            ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-${node}-${suffix} 2>&1
            Should Be Equal As Integers    ${rc}    0    msg=${node}-${suffix} not running
            Should Contain    ${out}    true
        END
    END

    # Lab dir and mgmt network should be preserved
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    test -d ${EXECDIR}/lab-examples/sr-sim/clab-${lab-name}
    Should Be Equal As Integers    ${rc}    0

    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker network inspect srsim_mgmt 2>&1
    Should Be Equal As Integers    ${rc}    0

Deploy all srsim10 nodes
    [Documentation]    Deploy all 4 srsim10 containers with node-filter. Network namespace sharing verified.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file} --node-filter srsim10-a,srsim10-b,srsim10-1,srsim10-2
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # All 4 srsim10 containers should be running
    FOR    ${suffix}    IN    a    b    1    2
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} not running
        Should Contain    ${out}    true
    END

    # srsim10-b, srsim10-1, srsim10-2 should use network-mode: container:srsim10-a
    FOR    ${suffix}    IN    b    1    2
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.HostConfig.NetworkMode}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0
        Should Contain    ${out}    container:clab-${lab-name}-srsim10-a
    END

Redeploy full lab for NS auto-expand test
    [Documentation]    Redeploy the full lab to set up the auto-expand namespace dependents test.
    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --cleanup
    ...    shell=True

    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file}
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

Destroy CPM-A auto-expands to include dependents
    [Documentation]
    ...    Destroy with --node-filter srsim10-a. The filter should auto-expand to include
    ...    srsim10-b, srsim10-1, srsim10-2 because they share srsim10-a's network namespace.
    ...    All 4 srsim10 containers should be destroyed. srsim11 should be untouched.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${lab-file} --node-filter srsim10-a
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Verify auto-expansion happened
    Should Contain    ${output.stderr}    Auto-including node

    # All 4 srsim10 containers should be gone
    FOR    ${suffix}    IN    a    b    1    2
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Not Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} still exists
    END

    # All 4 srsim11 containers should still be running
    FOR    ${suffix}    IN    a    b    1    2
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim11-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim11-${suffix} not running
        Should Contain    ${out}    true
    END


Redeploy for CPM-B test
    [Documentation]    Redeploy srsim10 nodes to set up CPM-B directional test.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab-file} --node-filter srsim10-a,srsim10-b,srsim10-1,srsim10-2
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

Destroy CPM-B does not auto-expand
    [Documentation]
    ...    Destroy with --node-filter srsim10-b. No other node depends on srsim10-b's namespace,
    ...    so the filter should NOT expand. Only srsim10-b is destroyed.
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

    # srsim10-a, srsim10-1, srsim10-2 should still be running
    FOR    ${suffix}    IN    a    1    2
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} not running
        Should Contain    ${out}    true
    END

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
