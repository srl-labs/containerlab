*** Settings ***
Library             OperatingSystem
Library             Collections
Resource            ../../common.robot

Suite Setup         Setup
Suite Teardown      Cleanup
Test Setup          Should Be Empty    ${labs}
Test Teardown       Destroy Labs


*** Variables ***
${runtime}          docker
${runtime-cli}      docker
${topo}             ${CURDIR}/34-mgmt-ipam.clab.yml
${vars-dir}         ${CURDIR}/clab-smoke34-vars
@{labs}


*** Test Cases ***
Containerlab IPAM allocates dual-stack management addresses
    Deploy Lab    34-single
    ${ipv4}    ${ipv6} =    Node Addresses    clab-34-single-r1
    Address Should Be In Pool    ${ipv4}    198.18.34.128/25
    Address Should Be In Pool    ${ipv6}    fd00:34::/120

Management addresses remain stable across redeployment
    Deploy Lab    34-sticky    narrow=${True}
    ${ipv4}    ${ipv6} =    Node Addresses    clab-34-sticky-r1
    Address Should Be In Pool    ${ipv4}    198.18.34.192/26
    Address Should Be In Pool    ${ipv6}    fd00:34::80/121
    Destroy Lab Preserving State    34-sticky
    Deploy Lab    34-sticky
    ${current4}    ${current6} =    Node Addresses    clab-34-sticky-r1
    Should Be Equal    ${ipv4}    ${current4}
    Should Be Equal    ${ipv6}    ${current6}

Shared management network reserves addresses owned by other labs
    Deploy Lab    34-shared-a
    ${a4}    ${a6} =    Node Addresses    clab-34-shared-a-r1
    Deploy Lab    34-shared-b
    ${b4}    ${b6} =    Node Addresses    clab-34-shared-b-r1
    Should Not Be Equal    ${a4}    ${b4}
    Should Not Be Equal    ${a6}    ${b6}
    Destroy Lab    34-shared-a
    ${current4}    ${current6} =    Node Addresses    clab-34-shared-b-r1
    Should Be Equal    ${b4}    ${current4}
    Should Be Equal    ${b6}    ${current6}


*** Keywords ***
Setup
    IF    '${runtime}' == 'podman'
        Set Suite Variable    ${runtime-cli}    sudo podman
    END
    Directory Should Not Exist    ${vars-dir}
    Create Directory    ${vars-dir}

Deploy Lab
    [Arguments]    ${name}    ${narrow}=${False}
    ${vars} =    Evaluate    json.dumps(dict(name=$name, narrow=$narrow))    modules=json
    Create File    ${vars-dir}/${name}.json    ${vars}
    Append To List    ${labs}    ${name}
    Command Should Succeed
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo} --vars ${vars-dir}/${name}.json

Destroy Lab
    [Arguments]    ${name}
    Command Should Succeed
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --vars ${vars-dir}/${name}.json --cleanup
    Remove Values From List    ${labs}    ${name}

Destroy Lab Preserving State
    [Arguments]    ${name}
    Command Should Succeed
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --vars ${vars-dir}/${name}.json
    Remove Values From List    ${labs}    ${name}

Destroy Labs
    ${remaining} =    Copy List    ${labs}
    FOR    ${name}    IN    @{remaining}
        Run Keyword And Continue On Failure    Destroy Lab    ${name}
    END

Cleanup
    Run Keyword And Continue On Failure    Destroy Labs
    Remove Directory    ${vars-dir}    recursive=True

Node Addresses
    [Arguments]    ${node}
    ${output} =    Command Should Succeed    ${runtime-cli} exec ${node} ip -o addr show eth0
    ${ipv4} =    Evaluate
    ...    next(line.split()[3].split('/')[0] for line in $output.splitlines() if ' inet ' in line and ' scope global ' in line)
    ${ipv6} =    Evaluate
    ...    next(line.split()[3].split('/')[0] for line in $output.splitlines() if ' inet6 ' in line and ' scope global ' in line)
    RETURN    ${ipv4}    ${ipv6}

Address Should Be In Pool
    [Arguments]    ${address}    ${pool}
    ${in-pool} =    Evaluate
    ...    ipaddress.ip_address($address) in ipaddress.ip_network($pool)
    ...    modules=ipaddress
    Should Be True    ${in-pool}

Command Should Succeed
    [Arguments]    ${command}
    ${rc}    ${output} =    Run And Return Rc And Output    ${command} 2>&1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0    ${command}: ${output}
    RETURN    ${output}
