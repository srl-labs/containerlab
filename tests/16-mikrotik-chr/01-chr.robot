*** Settings ***
Library             OperatingSystem
Library             SSHLibrary
Resource            ../common.robot
Resource            ../ssh.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         16-01-chr
${lab-file-name}    01-chr.clab.yml
${runtime}          docker
${redeploy_image}   %{CHR_REDEPLOY_IMAGE=}


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    IF    ${rc} != 0    Fatal Error    Failed to deploy ${lab-name} lab

Wait for r1 and r2 to be healthy
    Wait Until Keyword Succeeds    300s    5s    Node is healthy    r1
    Wait Until Keyword Succeeds    60s    5s    Node is healthy    r2

Ensure r1 is reachable over ssh with the default credentials and a lab default route
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r1
    ...    username=admin
    ...    password=admin
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    /system identity print
    Log    ${output}
    Should Contain    ${output}    name: r1
    ${output} =    SSHLibrary.Execute Command    /ping 10.0.0.2 count=3
    Log    ${output}
    Should Contain    ${output}    received=3
    SSHLibrary.Close Connection

Ensure r1 carries the containerlab management address on ether1
    ${rc}    ${mgmt-ip} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' clab-${lab-name}-r1
    Should Be Equal As Integers    ${rc}    0
    Login via SSH with username and password
    ...    address=${mgmt-ip}
    ...    username=admin
    ...    password=admin
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    /ip address print terse where interface=ether1
    Log    ${output}
    Should Contain    ${output}    address=${mgmt-ip}/
    SSHLibrary.Close Connection

Ensure r2 is reachable over ssh with custom credentials
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r2
    ...    username=netops
    ...    password=chr pass#1
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    /user print terse where name=admin
    Log    ${output}
    Should Contain    ${output}    X
    SSHLibrary.Close Connection

Ensure discovery uses data ports and excludes management by default
    Wait Until Keyword Succeeds    90s    5s    Discovery uses data port    r1    admin    admin
    Wait Until Keyword Succeeds    90s    5s    Discovery uses data port    r2    netops    chr pass#1

Ensure the rejected startup config of r3 is reported and r3 never becomes healthy
    Wait Until Keyword Succeeds    120s    5s    Node log contains    r3    startup config import FAILED
    ${output} =    Node log    r3
    Should Contain    ${output}    bad parameter network-type (line 2
    Should Not Contain    ${output}    RouterOS ready
    ${rc}    ${status} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} inspect -f '{{.State.Health.Status}}' clab-${lab-name}-r3
    Should Not Be Equal As Strings    ${status}    healthy

Save a custom discovery policy without the launcher's management objects
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r2
    ...    username=netops
    ...    password=chr pass#1
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    :put [/ip neighbor discovery-settings get discover-interface-list]
    Should Be Equal As Strings    ${output.strip()}    clab-discovery
    SSHLibrary.Execute Command    /interface list add name="data ports" include=static
    SSHLibrary.Execute Command    /ip neighbor discovery-settings set discover-interface-list="data ports"
    SSHLibrary.Execute Command    /ip vrf add name="user vrf" interfaces=none
    SSHLibrary.Execute Command    /ip service set winbox vrf="user vrf"
    SSHLibrary.Close Connection
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} save -t ${CURDIR}/${lab-file-name} --node-filter r2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/r2/ftpboot/config.auto.rsc
    Log    ${f}
    Should Contain    ${f}    10.0.0.2/30
    Should Not Contain    ${f}    routing-table=mgmt
    Should Not Contain    ${f}    name=mgmt
    Should Not Contain    ${f}    comment=clab-mgmt
    Should Contain    ${f}    discover-interface-list="data ports"
    Should Not Contain    ${f}    dying-gasp=

Ensure discovery and VRF overrides survive save and clean redeploy
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r1
    ...    username=admin
    ...    password=admin
    ...    try_for=30
    SSHLibrary.Execute Command    /ip neighbor discovery-settings set discover-interface-list=static
    SSHLibrary.Execute Command    /ip service set winbox vrf=main
    SSHLibrary.Execute Command    /ip service set api-ssl vrf=main disabled=yes
    SSHLibrary.Execute Command    /snmp set vrf=main
    SSHLibrary.Execute Command    /snmp community add name=save-check addresses=10.0.0.0/30
    SSHLibrary.Close Connection
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} save -t ${CURDIR}/${lab-file-name} --node-filter r1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${f} =    OperatingSystem.Get File    ${CURDIR}/clab-${lab-name}/r1/ftpboot/config.auto.rsc
    Log    ${f}
    Should Contain    ${f}    10.0.0.1/30
    Should Not Contain    ${f}    routing-table=mgmt
    Should Not Contain    ${f}    name=mgmt
    Should Not Contain    ${f}    comment=clab-mgmt
    Should Not Contain    ${f}    available-from=
    Should Not Contain    ${f}    clab-service-defaults
    Should Contain    ${f}    /ip service set [find where name=winbox dynamic=no] vrf=main
    Should Contain    ${f}    /snmp set vrf=main
    Should Contain    ${f}    /ip neighbor discovery-settings set discover-interface-list=static
    Should Not Contain    ${f}    dying-gasp=
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    IF    $redeploy_image
        Set Environment Variable    CHR_IMAGE    ${redeploy_image}
    END
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Wait Until Keyword Succeeds    300s    5s    Node is healthy    r1
    Wait Until Keyword Succeeds    300s    5s    Node is healthy    r2
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r1
    ...    username=admin
    ...    password=admin
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    :put [/ip neighbor discovery-settings get discover-interface-list]
    Should Be Equal As Strings    ${output.strip()}    static
    ${output} =    SSHLibrary.Execute Command    /interface list print count-only where name=clab-management
    Should Be Equal As Strings    ${output.strip()}    1
    ${output} =    SSHLibrary.Execute Command    /interface list print count-only where name=clab-discovery
    Should Be Equal As Strings    ${output.strip()}    1
    ${output} =    SSHLibrary.Execute Command    /ip address print terse where interface=ether2
    Should Contain    ${output}    address=10.0.0.1/30
    ${output} =    SSHLibrary.Execute Command    :put [/ip service get winbox vrf]
    Should Be Equal As Strings    ${output.strip()}    main
    ${output} =    SSHLibrary.Execute Command    :put [/ip service get api-ssl vrf]
    Should Be Equal As Strings    ${output.strip()}    main
    ${output} =    SSHLibrary.Execute Command    :put [/ip service get api-ssl disabled]
    Should Be Equal As Strings    ${output.strip()}    true
    ${output} =    SSHLibrary.Execute Command    :put [/snmp get vrf]
    Should Be Equal As Strings    ${output.strip()}    main
    ${output} =    SSHLibrary.Execute Command    /snmp community print count-only where name=save-check
    Should Be Equal As Strings    ${output.strip()}    1
    ${output} =    SSHLibrary.Execute Command    /ip vrf print count-only where name=mgmt
    Should Be Equal As Strings    ${output.strip()}    1
    ${output} =    SSHLibrary.Execute Command    /ping 10.0.0.2 count=3
    Should Contain    ${output}    received=3
    SSHLibrary.Close Connection
    Login via SSH with username and password
    ...    address=clab-${lab-name}-r2
    ...    username=netops
    ...    password=chr pass#1
    ...    try_for=30
    ${output} =    SSHLibrary.Execute Command    :put [/ip neighbor discovery-settings get discover-interface-list]
    Should Be Equal As Strings    ${output.strip()}    data ports
    ${output} =    SSHLibrary.Execute Command    /ping 10.0.0.1 count=3
    Should Contain    ${output}    received=3
    ${output} =    SSHLibrary.Execute Command    :put [/ip service get winbox vrf]
    Should Be Equal As Strings    ${output.strip()}    user vrf
    ${output} =    SSHLibrary.Execute Command    /ip vrf print count-only where name="user vrf"
    Should Be Equal As Strings    ${output.strip()}    1
    SSHLibrary.Close Connection


*** Keywords ***
Discovery uses data port
    [Arguments]    ${node}    ${username}    ${password}
    Login via SSH with username and password
    ...    address=clab-${lab-name}-${node}
    ...    username=${username}
    ...    password=${password}
    ...    try_for=10
    TRY
        ${output} =    SSHLibrary.Execute Command    /ip neighbor print count-only where interface=ether1
        Should Be Equal As Strings    ${output.strip()}    0
        ${output} =    SSHLibrary.Execute Command    /ip neighbor print count-only where interface=ether2
        Should Be Equal As Strings    ${output.strip()}    1
    FINALLY
        SSHLibrary.Close Connection
    END

Node is healthy
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} inspect -f '{{.State.Health.Status}}' clab-${lab-name}-${node}
    Log    ${output}
    Should Be Equal As Strings    ${output}    healthy

Node log
    [Arguments]    ${node}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo -E ${runtime} logs clab-${lab-name}-${node}
    RETURN    ${output}

Node log contains
    [Arguments]    ${node}    ${text}
    ${output} =    Node log    ${node}
    Should Contain    ${output}    ${text}

Cleanup
    FOR    ${node}    IN    r1    r2    r3
        ${output} =    Node log    ${node}
        Log    ${output}
    END
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
