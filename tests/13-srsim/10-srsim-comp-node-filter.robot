*** Settings ***
Library             Process
Library             OperatingSystem
Resource            ../common.robot

Suite Teardown      Cleanup


*** Variables ***
${lab-file-name}    10-srsim-comp-node-filter.clab.yml
${lab-name}         sr10
${runtime}          docker


*** Test Cases ***
Deploy full components lab
    [Documentation]    Deploy the distributed components lab with srsim10 and srsim11 (2 components each).
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # All 4 component containers should be running (2 per node)
    FOR    ${node}    IN    srsim10    srsim11
        FOR    ${suffix}    IN    a    1
            ${rc}    ${out} =    Run And Return Rc And Output
            ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-${node}-${suffix} 2>&1
            Should Be Equal As Integers    ${rc}    0    msg=${node}-${suffix} not running
            Should Contain    ${out}    true
        END
    END

Destroy with node-filter srsim10
    [Documentation]    Destroy srsim10. Both srsim10 containers removed. srsim11 untouched.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --node-filter srsim10
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Both srsim10 component containers should be gone
    FOR    ${suffix}    IN    a    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Not Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} still exists
    END

    # Both srsim11 component containers should still be running
    FOR    ${suffix}    IN    a    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim11-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim11-${suffix} not running
        Should Contain    ${out}    true
    END

    # Lab directory should be preserved
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    test -d ${CURDIR}/clab-${lab-name}
    Should Be Equal As Integers    ${rc}    0

    # Inspect should still work with partial lab (srsim11 only)
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} inspect -t ${CURDIR}/${lab-file-name}
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stdout}    srsim11
    Should Not Contain    ${output.stdout}    srsim10

Deploy with node-filter srsim10
    [Documentation]    Redeploy srsim10 into the existing lab. Both containers created.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name} --node-filter srsim10
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # Both srsim10 containers should be running again
    FOR    ${suffix}    IN    a    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim10-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim10-${suffix} not running
        Should Contain    ${out}    true
    END

    # srsim10-a should have the management IP
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAMConfig.IPv4Address}}{{end}}' clab-${lab-name}-srsim10-a 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${out}    10.78.150.2

    # Line card should share network namespace of srsim10-a
    ${rc}    ${out} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.HostConfig.NetworkMode}}' clab-${lab-name}-srsim10-1 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${out}    container:clab-${lab-name}-srsim10-a

Reconfigure with node-filter srsim11
    [Documentation]    Reconfigure srsim11 only. srsim10 untouched, srsim11 redeployed fresh.
    # Capture srsim10-a container ID before to verify it wasn't recreated
    ${rc}    ${id_before} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.Id}}' clab-${lab-name}-srsim10-a 2>&1
    Should Be Equal As Integers    ${rc}    0

    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name} --reconfigure --node-filter srsim11
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    # srsim10-a should be the same container
    ${rc}    ${id_after} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{.Id}}' clab-${lab-name}-srsim10-a 2>&1
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal    ${id_before}    ${id_after}

    # Both srsim11 containers should be running (freshly redeployed)
    FOR    ${suffix}    IN    a    1
        ${rc}    ${out} =    Run And Return Rc And Output
        ...    sudo docker inspect -f '{{.State.Running}}' clab-${lab-name}-srsim11-${suffix} 2>&1
        Should Be Equal As Integers    ${rc}    0    msg=srsim11-${suffix} not running
        Should Contain    ${out}    true
    END


*** Keywords ***
Cleanup
    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    ...    shell=True
