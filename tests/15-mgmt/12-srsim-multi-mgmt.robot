*** Settings ***
Documentation       SR-SIM nodes on separate management networks.
...                 Each node must use the gateway of its own management network,
...                 so SSH sessions sourced from the main network only work
...                 when the SR OS management route points to the right gateway.
Library             OperatingSystem
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Run Keyword And Ignore Error    Destroy Lab    ${moved-vars}


*** Variables ***
${runtime}          docker
${topo}             ${CURDIR}/12-srsim-multi-mgmt.clab.yml
${initial-vars}     ${CURDIR}/12-srsim-multi-mgmt.vars.initial.yml
${moved-vars}       ${CURDIR}/12-srsim-multi-mgmt.vars.moved.yml
${license}          /opt/nokia/sros/license.txt
# host address on the main management network, used as the SSH source address
${main-gw}          10.78.150.1


*** Test Cases ***
Deploy SR-SIM nodes on three management networks
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo} --vars ${initial-vars}

Integrated node is attached to its management network
    ${ip} =    Node Address    clab-sr13-sr-int    sr13-int
    Should Be Equal    ${ip}    10.78.151.10
    ${ip} =    Node Address    clab-sr13-sr-int    sr13-int    GlobalIPv6Address
    Should Be Equal    ${ip}    fd00:10:78:151::10

Distributed node namespace is attached to its management network
    ${ip} =    Node Address    clab-sr13-sr-dist-netns    sr13-dist
    Should Start With    ${ip}    10.78.152.
    Set Suite Variable    ${dist-ip}    ${ip}
    FOR    ${component}    IN    clab-sr13-sr-dist-a    clab-sr13-sr-dist-1
        ${mode} =    Command Should Succeed
        ...    docker inspect -f '{{.HostConfig.NetworkMode}}' ${component}
        Should Start With    ${mode}    container:
    END

SR OS uses the gateway of its own management network
    Wait Until Keyword Succeeds    5 min    10s
    ...    Management Route Should Use Gateway    10.78.151.10    10.78.151.1
    Wait Until Keyword Succeeds    5 min    10s
    ...    Management Route Should Use Gateway    ${dist-ip}    10.78.152.1

Data link between linux and SR-SIM works
    Wait Until Keyword Succeeds    3 min    10s
    ...    Command Should Succeed    docker exec clab-sr13-l1 ping -c 1 -W 2 10.0.0.2

Apply moves the integrated node to another management network
    ${output} =    Command Should Succeed
    ...    ${CLAB_BIN} --runtime ${runtime} apply -t ${topo} --vars ${moved-vars}
    Should Contain    ${output}    MgmtNet
    ${ip} =    Node Address    clab-sr13-sr-int    sr13-dist
    Should Start With    ${ip}    10.78.152.
    Wait Until Keyword Succeeds    5 min    10s
    ...    Management Route Should Use Gateway    ${ip}    10.78.152.1

Destroy removes the nodes and every management network
    Destroy Lab    ${moved-vars}
    ${containers} =    Command Should Succeed
    ...    docker ps -a --filter label=containerlab=sr13 --format '{{.Names}}'
    Should Be Empty    ${containers}
    ${networks} =    Command Should Succeed    docker network ls --format '{{.Name}}'
    Should Not Contain    ${networks}    sr13-


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    Multiple management networks require Docker.
    ${licensed} =    Run Keyword And Return Status    File Should Exist    ${license}
    Skip If    not ${licensed}    SR-SIM license ${license} is not available.

Destroy Lab
    [Arguments]    ${vars}
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --vars ${vars} --cleanup

Management Route Should Use Gateway
    [Documentation]    Logs in from the main network and checks the SR OS management default route.
    [Arguments]    ${address}    ${gateway}
    ${output} =    Command Should Succeed
    ...    echo "show router management route-table" | sshpass -p 'NokiaSros1!' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o IdentitiesOnly=yes -o ConnectTimeout=5 -b ${main-gw} admin@${address}
    Should Match Regexp    ${output}    0\\.0\\.0\\.0/0[^\\n]*\\n\\s*${gateway.replace('.', '\\.')}\\s
