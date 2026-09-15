*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         srsim-pause-ctr
${lab-file-name}    09-srsim-pause-container.clab.yml
${runtime}          docker
${gnmic_image}      ghcr.io/openconfig/gnmic:0.42.1
${gnmic_flags}      --username admin --password NokiaSros1! --values-only --insecure


*** Test Cases ***
Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Wait for 45s
    Sleep    45s    Let everything fully provision & come up

# IPv4
Check SR-14s inspect returns IPv4 mgmt IP correctly
    ${rc}    ${output} =    Run Checked Pipeline
    ...    ${CLAB_BIN} --runtime ${runtime} ins -t ${CURDIR}/${lab-file-name} -f json | jq -r '.["${lab-name}"][] | select(.name == "clab-${lab-name}-sr14s-a") | .ipv4_address | split("/")[0]'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${valid} =    Evaluate    ipaddress.ip_address($output) in ipaddress.ip_network("172.20.20.0/24")    modules=ipaddress
    Should Be True    ${valid}
    Set Suite Variable    ${sr14s-ipv4}    ${output}

Check SR-14s namespace holder owns mgmt IPv4 addr
    ${rc}    ${output} =    Run Checked Pipeline
    ...    docker inspect clab-${lab-name}-sr14s-netns | jq -r '.[0].NetworkSettings.Networks[].IPAddress'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    ${sr14s-ipv4}

Confirm SR-14s component doesn't have any IPv4 address
    ${rc}    ${output} =    Run Checked Pipeline
    ...    ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["${lab-name}"][] | select(.name == "clab-${lab-name}-sr14s-a") | .ipv4_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    N/A

# IPv6
Check SR-14s inspect returns IPv6 mgmt IP correctly
    ${rc}    ${output} =    Run Checked Pipeline
    ...    ${CLAB_BIN} --runtime ${runtime} ins -t ${CURDIR}/${lab-file-name} -f json | jq -r '.["${lab-name}"][] | select(.name == "clab-${lab-name}-sr14s-a") | .ipv6_address | split("/")[0]'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    ${valid} =    Evaluate    ipaddress.ip_address($output) in ipaddress.ip_network("3fff:172:20:20::/64")    modules=ipaddress
    Should Be True    ${valid}
    Set Suite Variable    ${sr14s-ipv6}    ${output}

Check SR-14s namespace holder owns mgmt IPv6 addr
    ${rc}    ${output} =    Run Checked Pipeline
    ...    docker inspect clab-${lab-name}-sr14s-netns | jq -r '.[0].NetworkSettings.Networks[].GlobalIPv6Address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    ${sr14s-ipv6}

Confirm SR-14s component doesn't have any IPv6 address
    ${rc}    ${output} =    Run Checked Pipeline
    ...    ${CLAB_BIN} --runtime ${runtime} ins -a -f json | jq -r '.["${lab-name}"][] | select(.name == "clab-${lab-name}-sr14s-a") | .ipv6_address'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    N/A

Confirm topology inspect hides namespace holder
    ${rc}    ${output} =    Run Checked Pipeline
    ...    ${CLAB_BIN} --runtime ${runtime} ins -t ${CURDIR}/${lab-file-name} -f json | jq '[.["${lab-name}"][] | .name] == ["clab-${lab-name}-sr14s-a"]'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    true

Confirm inspect all includes namespace holder
    FOR    ${selector}    IN    --all    --name ${lab-name}
        FOR    ${format}    IN    -f json    --details
            ${rc}    ${output} =    Run And Return Rc And Output
            ...    ${CLAB_BIN} --runtime ${runtime} ins ${selector} ${format}
            Should Be Equal As Integers    ${rc}    0
            Should Contain    ${output}    clab-${lab-name}-sr14s-a
            IF    $selector == '--all'
                Should Contain    ${output}    clab-${lab-name}-sr14s-netns
            ELSE
                Should Not Contain    ${output}    clab-${lab-name}-sr14s-netns
            END
        END
    END

CPM B maintains SSH access after CPM A is killed
    Wait Until Keyword Succeeds    120s    3s    Check CPM SSH    A
    ${rc}    ${cpm-b-started} =    Run And Return Rc And Output
    ...    docker inspect -f '{{.State.StartedAt}}' clab-${lab-name}-sr14s-b
    Should Be Equal As Integers    ${rc}    0

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    docker kill clab-${lab-name}-sr14s-a
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    Wait Until Keyword Succeeds    120s    3s    Check CPM SSH    B
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    docker inspect -f '{{.State.Running}}' clab-${lab-name}-sr14s-a
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    false
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    docker inspect -f '{{.State.StartedAt}}' clab-${lab-name}-sr14s-b
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    ${cpm-b-started}

Remove components leaving only the holder
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    docker rm -f clab-${lab-name}-sr14s-a clab-${lab-name}-sr14s-b clab-${lab-name}-sr14s-1 clab-${lab-name}-sr14s-2
    Should Be Equal As Integers    ${rc}    0

Destroy holder-only lab using topology
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    docker inspect -f '{{.State.Running}}' clab-${lab-name}-sr14s-netns
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    true
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    docker ps -a --filter label=containerlab=${lab-name} --format '{{.Names}}'
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal As Strings    ${output}    clab-${lab-name}-sr14s-netns
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Should Be Equal As Integers    ${rc}    0
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    docker ps -a --filter name=clab-${lab-name}-sr14s --format '{{.Names}}'
    Should Be Equal As Integers    ${rc}    0
    Should Be Empty    ${output}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip netns list
    Should Be Equal As Integers    ${rc}    0
    Should Not Contain    ${output}    clab-${lab-name}-sr14s

*** Keywords ***
Run Checked Pipeline
    [Arguments]    ${command}
    ${result} =    Process.Run Process    bash    -o    pipefail    -c    ${command}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0    ${result.stderr}
    [Return]    ${result.rc}    ${result.stdout}

Check CPM SSH
    [Arguments]    ${slot}
    TRY
        SSHLibrary.Open Connection
        ...    ${sr14s-ipv4}
        ...    timeout=5s
        ...    prompt=REGEXP:${slot}:admin@[^\\r\\n]+#
        SSHLibrary.Login    admin    NokiaSros1!
        SSHLibrary.Write    show version
        ${output} =    SSHLibrary.Read Until Prompt
        Log    ${output}
        Should Contain    ${output}    TiMOS
        Should Match Regexp    ${output}    ${slot}:admin@[^\\r\\n]+#
    FINALLY
        SSHLibrary.Close All Connections
    END

Cleanup
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
