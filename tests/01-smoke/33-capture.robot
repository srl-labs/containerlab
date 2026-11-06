*** Comments ***
This suite tests the capture command with the local container runtimes: containerlab reads the
lab link interfaces through packet sockets in the nodes' network namespaces.


*** Settings ***
Library             OperatingSystem
Library             Process
Library             String
Library             Collections
Resource            ../common.robot

Suite Setup         Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}         capture
${topo}             ${CURDIR}/33-capture.clab.yml
${runtime}          docker
${summary-script}   ${CURDIR}/33-capture-summary.py
${pcap}             /tmp/clab-capture.pcapng
${stdout-pcap}      /tmp/clab-capture-stdout.pcapng
${interrupt-pcap}   /tmp/clab-capture-interrupt.pcapng
${restart-pcap}     /tmp/clab-capture-restart.pcapng


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

List the capturable interfaces
    ${result} =    Run Capture    --name ${lab-name} --list -f json
    Should Be Equal As Integers    ${result.rc}    0
    ${interfaces} =    Evaluate    json.loads($result.stdout)    modules=json
    Length Should Be    ${interfaces}    3
    Should Be Equal    ${interfaces}[0][node]    l1
    Should Be Equal    ${interfaces}[0][interface]    eth1
    Should Be Equal    ${interfaces}[0][peer_node]    l2
    Should Be Equal    ${interfaces}[2][node]    l2
    Should Be Equal    ${interfaces}[2][interface]    eth2
    Should Be Equal    ${interfaces}[2][peer_node]    host
    Should Be Equal    ${interfaces}[2][peer_interface]    capl2eth2

Capture both ends of a link with VLAN traffic
    Start Process
    ...    ${CLAB_BIN} --runtime ${runtime} capture --name ${lab-name} --target l1:eth1 --target l2:eth1 --duration 8s -w ${pcap}
    ...    shell=True
    ...    alias=both-ends
    Sleep    2s
    Ping From L1    10.0.0.2
    Ping From L1    10.10.0.2
    ${result} =    Wait For Process    both-ends    timeout=30s    on_timeout=kill
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    ${summary} =    Capture Summary    ${pcap}
    Should Be Equal As Integers    ${summary}[sections]    1
    ${want} =    Create List    l1:eth1    l2:eth1
    Lists Should Be Equal    ${summary}[interfaces]    ${want}
    Should Be True    ${summary}[ordered]
    # 6 pings, each request and reply seen on both ends
    Should Be True    ${summary}[icmp_echo] >= 24
    # the VLAN 10 pings keep their 802.1Q tag
    Should Be True    ${summary}[vlan_frames] >= 12

Capture to stdout stops at the packet limit
    Start Process
    ...    sleep 1 && sudo ${runtime} exec clab-${lab-name}-l1 ping -c 20 -i 0.2 10.0.0.2
    ...    shell=True
    ...    alias=background-ping
    ${result} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} capture --name ${lab-name} --target l2:eth1 --packet-limit 6 -w - > ${stdout-pcap}
    ...    shell=True
    ...    timeout=30s
    ...    on_timeout=kill
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    ${summary} =    Capture Summary    ${stdout-pcap}
    Should Be Equal As Integers    ${summary}[packets][l2:eth1]    6
    [Teardown]    Terminate Process    background-ping    kill=True

An interrupt ends the capture with a valid file
    Start Process
    ...    exec ${CLAB_BIN} --runtime ${runtime} capture --name ${lab-name} --target l1:eth1 -w ${interrupt-pcap}
    ...    shell=True
    ...    alias=interrupted
    Sleep    2s
    Ping From L1    10.0.0.2
    Send Signal To Process    SIGINT    interrupted
    ${result} =    Wait For Process    interrupted    timeout=15s    on_timeout=kill
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    ${summary} =    Capture Summary    ${interrupt-pcap}
    Should Be True    ${summary}[icmp_echo] >= 6

The capture reattaches an interface after its node restarts
    Start Process
    ...    exec ${CLAB_BIN} --runtime ${runtime} capture --name ${lab-name} --target l2:eth1 -w ${restart-pcap}
    ...    shell=True
    ...    alias=restarted
    Sleep    2s
    Ping From L1    10.0.0.2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} restart -t ${topo} --node l2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} exec clab-${lab-name}-l2 ip addr replace 10.0.0.2/24 dev eth1
    Log    ${output}
    Wait Until Keyword Succeeds    20s    1s    Ping From L1    10.0.0.2
    # the capture reattaches within a fraction of a second after the link is back
    Sleep    1s
    Ping From L1    10.0.0.2
    Send Signal To Process    SIGINT    restarted
    ${result} =    Wait For Process    restarted    timeout=15s    on_timeout=kill
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stderr}    Capture has a gap
    ${summary} =    Capture Summary    ${restart-pcap}
    # 3 pings before and 3 after the reattach, request and reply each
    Should Be True    ${summary}[icmp_echo] >= 12

The capture rejects the management interface
    ${result} =    Run Capture    --name ${lab-name} --target l1:eth0 -w /dev/null
    Should Not Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stderr}    not a capturable link interface; capturable: eth1


*** Keywords ***
Run Capture
    [Arguments]    ${args}
    ${result} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} capture ${args}
    ...    shell=True
    ...    timeout=60s
    ...    on_timeout=kill
    Log    stdout:${\n}${result.stdout}
    Log    stderr:${\n}${result.stderr}
    RETURN    ${result}

Ping From L1
    [Arguments]    ${address}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} exec clab-${lab-name}-l1 ping -c 3 -i 0.2 -W 1 ${address}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Capture Summary
    [Arguments]    ${file}
    ${rc}    ${output} =    Run And Return Rc And Output    python3 ${summary-script} ${file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${summary} =    Evaluate    json.loads($output)    modules=json
    RETURN    ${summary}

Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    Run    sudo rm -f ${pcap} ${stdout-pcap} ${interrupt-pcap} ${restart-pcap}
