*** Settings ***
Documentation       Keywords shared by the management network suites.
Library             OperatingSystem


*** Keywords ***
Command Should Succeed
    [Arguments]    ${command}
    ${rc}    ${output} =    Run And Return Rc And Output    ${command} 2>&1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0    ${command}: ${output}
    RETURN    ${output}

Command Should Fail
    [Arguments]    ${command}
    ${rc}    ${output} =    Run And Return Rc And Output    ${command} 2>&1
    Log    ${output}
    Should Not Be Equal As Integers    ${rc}    0    ${command} unexpectedly succeeded: ${output}
    RETURN    ${output}

Node Address
    [Documentation]    Returns an endpoint field (IPAddress, GlobalIPv6Address, ...) of a container on a network.
    [Arguments]    ${container}    ${network}    ${field}=IPAddress
    ${output} =    Command Should Succeed
    ...    docker inspect -f '{{(index .NetworkSettings.Networks "${network}").${field}}}' ${container}
    RETURN    ${output}

Address Should Be In Pool
    [Arguments]    ${address}    ${pool}
    ${in-pool} =    Evaluate
    ...    ipaddress.ip_address($address) in ipaddress.ip_network($pool)
    ...    modules=ipaddress
    Should Be True    ${in-pool}    ${address} is not in ${pool}

Network Driver Should Be
    [Arguments]    ${network}    ${driver}
    ${output} =    Command Should Succeed    docker network inspect -f '{{.Driver}}' ${network}
    Should Be Equal    ${output}    ${driver}

Network Should Not Exist
    [Arguments]    ${network}
    ${output} =    Command Should Succeed    docker network ls --format '{{.Name}}'
    Should Not Contain    ${output.splitlines()}    ${network}

Network Bridge
    [Documentation]    Returns the linux bridge backing a docker bridge network.
    [Arguments]    ${network}
    ${id} =    Command Should Succeed    docker network inspect -f '{{.Id}}' ${network}
    ${name} =    Command Should Succeed
    ...    docker network inspect -f '{{index .Options "com.docker.network.bridge.name"}}' ${network}
    IF    '${name}' == '' or '${name}' == '<no value>'
        ${name} =    Set Variable    br-${id}[:12]
    END
    RETURN    ${name}

Containerlab Firewall Rule Count
    [Documentation]    Counts the forwarding rules containerlab installed for a bridge.
    [Arguments]    ${bridge}
    ${output} =    Command Should Succeed
    ...    { sudo iptables-save 2>/dev/null; sudo nft list ruleset 2>/dev/null; } | grep '${bridge}' | grep -ci 'set by containerlab' || true
    RETURN    ${output}
