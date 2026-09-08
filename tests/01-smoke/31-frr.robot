*** Settings ***
Library             OperatingSystem
Resource            ../common.robot

Suite Setup         Deploy Lab
Suite Teardown      Cleanup


*** Variables ***
${lab-name}         frr-native
${topology}         ${CURDIR}/31-frr.clab.yml
${runtime}          docker
${runtime-cli}      sudo ${runtime}
${config-dir}       ${CURDIR}/clab-${lab-name}/r1/config


*** Test Cases ***
Startup configuration is rendered and mounted
    ${config} =    Get File    ${config-dir}/frr.conf
    Should Contain    ${config}    hostname r1
    Should Not Contain    ${config}    {{
    Wait Until Keyword Succeeds    60s    2s    Running Config Contains    r1    ip address 10.31.1.1/32
    ${vtysh} =    Node Command    r1    cat /etc/frr/vtysh.conf
    Should Contain    ${vtysh}    service integrated-vtysh-config

Daemon selection is inherited from the kind
    ${daemons} =    Node Command    r1    cat /etc/frr/daemons
    Should Contain    ${daemons}    ospfd=yes
    Should Contain    ${daemons}    bgpd=no
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli} exec clab-${lab-name}-r1 pgrep -x bgpd
    Should Be Equal As Integers    ${rc}    1

Alias and default configuration work without startup config
    Wait Until Keyword Succeeds    60s    2s    Running Config Contains    defaults    frr defaults traditional
    ${daemons} =    Node Command    defaults    cat /etc/frr/daemons
    Should Not Contain    ${daemons}    =no
    Should Contain    ${daemons}    bgpd=yes
    Should Contain    ${daemons}    ospfd=yes

Forwarding is enabled
    ${ipv4} =    Node Command    r1    sysctl -n net.ipv4.ip_forward
    Should Be Equal    ${ipv4}    1
    ${ipv6} =    Node Command    r1    sysctl -n net.ipv6.conf.all.forwarding
    Should Be Equal    ${ipv6}    1

OSPF installs a route to the remote loopback
    Wait Until Keyword Succeeds    90s    2s    OSPF Route Exists
    Node Command    r1    ping -c 3 -W 2 -I 10.31.1.1 10.31.2.1

Saved configuration survives redeployment
    Node Command    r1    vtysh -c 'configure terminal' -c 'interface lo' -c 'description saved-by-containerlab'
    Clab Command    save -t ${topology}
    ${config} =    Get File    ${config-dir}/frr.conf
    Should Contain    ${config}    description saved-by-containerlab
    Should Not Contain    ${config}    Building configuration
    Clab Command    destroy -t ${topology}
    Clab Command    deploy -t ${topology}
    Wait Until Keyword Succeeds    60s    2s
    ...    Running Config Contains    r1    description saved-by-containerlab

Enforced startup configuration replaces saved changes
    Clab Command    destroy -t ${topology}
    Set Environment Variable    FRR_ENFORCE_STARTUP_CONFIG    true
    Clab Command    deploy -t ${topology}
    Wait Until Keyword Succeeds    60s    2s    Running Config Contains    r1    ip address 10.31.1.1/32
    ${config} =    Node Command    r1    vtysh -c 'show running-config'
    Should Not Contain    ${config}    saved-by-containerlab


*** Keywords ***
Deploy Lab
    Set Environment Variable    FRR_ENFORCE_STARTUP_CONFIG    false
    Clab Command    deploy -t ${topology}

Cleanup
    Clab Command    destroy -t ${topology} --cleanup

Clab Command
    [Arguments]    ${args}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} ${args} 2>&1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

Node Command
    [Arguments]    ${node}    ${command}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli} exec clab-${lab-name}-${node} ${command} 2>&1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    RETURN    ${output}

Running Config Contains
    [Arguments]    ${node}    ${expected}
    ${config} =    Node Command    ${node}    vtysh -c 'show running-config'
    Should Contain    ${config}    ${expected}

OSPF Route Exists
    ${routes} =    Node Command    r1    vtysh -c 'show ip route ospf'
    Should Contain    ${routes}    10.31.2.1/32
