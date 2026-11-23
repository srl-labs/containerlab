*** Settings ***
Library             OperatingSystem
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}             03-04-ceos-mgmt-link
${lab-file-name}        04-ceos-mgmt-link-clab.yml
${invalid-lab-name}     03-05-ceos-mgmt-link-invalid
${invalid-lab-file}     05-ceos-mgmt-link-invalid-clab.yml
${runtime}              docker
${oob-mgmt-ip}          192.168.99.2
${peer-link-ip}         192.168.99.1


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait for dataplane to settle
    Sleep    10s

Verify oob management interface uses the wired link address
    [Documentation]
    ...    The management interface is wired as a link, so Management0 must come
    ...    up addressed from that link rather than from the management network.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=oob --cmd "Cli -p 15 -c 'show ip interface brief'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Management0
    Should Contain    ${output}    ${oob-mgmt-ip}

Ensure oob can ping its peer over the wired management link
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=oob --cmd "Cli -p 15 -c 'ping ${peer-link-ip} repeat 3'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Ensure oob is not attached to the management network
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo docker inspect clab-${lab-name}-oob -f '{{json .NetworkSettings.Networks}}'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    none

Ensure oob startup-config has no management default route
    [Documentation]
    ...    A detached node has no management network to route through, so no
    ...    default route via the management gateway should be templated.
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/oob/flash/startup-config
    Log    ${f}
    Should Not Contain    ${f}    ip route 0.0.0.0/0

Ensure ztp node boots without a startup-config
    [Documentation]
    ...    With suppress-startup-config the node has no startup-config, which is
    ...    what lets it actually zero-touch provision instead of booting fully
    ...    configured.
    OperatingSystem.File Should Not Exist    ${CURDIR}/clab-${lab-name}/ztp/flash/startup-config

Verify ztp management interface is wired but unaddressed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=ztp --cmd "Cli -p 15 -c 'show ip interface brief'"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Management0
    Should Not Contain    ${output}    ${oob-mgmt-ip}

Fail to deploy when eth0 is wired without network-mode none
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${invalid-lab-file}
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    network mode is not set to none


*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${invalid-lab-file} --cleanup
    Run    rm -rf ${CURDIR}/clab-${lab-name}
    Run    rm -rf ${CURDIR}/clab-${invalid-lab-name}
