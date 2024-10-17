*** Settings ***
Library             OperatingSystem
Library             String
Library             Collections
Library             Process
Resource            ../common.robot
Library             Collections

Suite Setup         Setup
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-file}     stages.clab.yml
${lab-name}     stages
${runtime}      docker


*** Test Cases ***
Pre-Pull Image
    ${output} =    Process.Run Process
    ...    ${runtime} pull debian:bookworm-slim
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

Deploy ${lab-name} lab
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    Set Suite Variable    ${deploylog}    ${output}

Ensure node3 started after node4
    [Documentation]    Ensure node3 is started after node4 since node3 waits for node4 to be healthy.
    ...    All containers write the unix timestamp whenever they are started to /tmp/time file and we compare the timestamps.
    ${node3} =    Process.Run Process
    ...    sudo -E docker exec clab-${lab-name}-node3 cat /tmp/time
    ...    shell=True

    Log    ${node3.stdout}
    Log    ${node3.stderr}

    ${node4} =    Process.Run Process
    ...    sudo -E docker exec clab-${lab-name}-node4 cat /tmp/time
    ...    shell=True

    Log    ${node4.stdout}
    Log    ${node4.stderr}

    ${n3} =    Convert To Integer    ${node3.stdout}
    ${n4} =    Convert To Integer    ${node4.stdout}

    Should Be True    ${n3} > ${n4}

Ensure node4 executed on-exit commands for its create-links stage and this output contains all interfaces
    ${match} =    Find Substring in Text
    ...    ${deploylog.stderr}
    ...    Executed command \\"ls /sys/class/net/\\" on the node \\"node4\\". stdout:\\neth0\\neth2\\neth3\\nlo\\n

    Log    ${match}

Ensure node4 executed on-exit commands for its heathy stage
    ${match} =    Find Substring in Text
    ...    ${deploylog.stderr}
    ...    Executed command \\"echo hey I am existing healthy stage\\" on the node \\"node4\\". stdout:\\nhey I am existing healthy stage\\n

    Log    ${match}

Ensure node3 executed on-exit commands for its create stage and this output doesn't contain any non eth0/lo interfaces
    ${match} =    Find Substring in Text
    ...    ${deploylog.stderr}
    ...    Executed command \\"ls /sys/class/net/\\" on the node \\"node3\\". stdout:\\neth0\\nlo\\n

    Should Not Contain    ${match}    eth1

    Log    ${match}

Ensure node1 executed on-enter commands for its create-links stage and this output doesn't contain any non eth0/lo interfaces
    ${match} =    Find Substring in Text
    ...    ${deploylog.stderr}
    ...    Executed command \\"ls /sys/class/net/\\" on the node \\"node1\\". stdout:\\neth0\\nlo\\n

    Should Not Contain    ${match}    eth1

    Log    ${match}

Ensure host-exec file is created with the right content
    ${content} =    Get File    /tmp/host-exec-test
    Should Contain    ${content}    foo    msg=File does not contain the expected string

Deploy ${lab-name} lab with a single worker
    Run Keyword    Teardown

    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy --max-workers 1 -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

Ensure node3 started after node4 after a single worker run
    [Documentation]    Ensure node3 is started after node4 since node3 waits for node4 to be healthy.
    ...    All containers write the unix timestamp whenever they are started to /tmp/time file and we compare the timestamps.
    ${node3} =    Process.Run Process
    ...    sudo -E docker exec clab-${lab-name}-node3 cat /tmp/time
    ...    shell=True

    Log    ${node3.stdout}
    Log    ${node3.stderr}

    ${node4} =    Process.Run Process
    ...    sudo -E docker exec clab-${lab-name}-node4 cat /tmp/time
    ...    shell=True

    Log    ${node4.stdout}
    Log    ${node4.stderr}

    ${n3} =    Convert To Integer    ${node3.stdout}
    ${n4} =    Convert To Integer    ${node4.stdout}

    Should Be True    ${n3} > ${n4}


*** Keywords ***
Teardown
    Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -c -t ${CURDIR}/${lab-file}

Setup
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -c -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}
    # skipping this test suite for podman for now
    Skip If    '${runtime}' == 'podman'

Find Substring in Text
    [Documentation]    Find a substring in a text and return the match object. Errors if the match is empty (not found)
    [Arguments]    ${text}    ${substring}

    @{lines} =    String.Split To Lines    ${text}
    Log    ${lines}

    ${match} =    Collections.Get Matches
    ...    ${lines}
    ...    *${substring}*
    Log    ${match}

    Should Not Be Empty    ${match}

    RETURN    ${match}
