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
${lab-file}                     stages.clab.yml
${lab-name}                     stages
${runtime}                      docker
${n4-exec-output}               SEPARATOR=\n
...                             Executed command node=node4 command="uname -n"
...                             ${SPACE}${SPACE}stdout=
...                             ${SPACE}${SPACE}│ node4
${n4-exec-healthy-output}       SEPARATOR=\n
...                             Executed command node=node4 command="echo hey I am exiting healthy stage"
...                             ${SPACE}${SPACE}stdout=
...                             ${SPACE}${SPACE}│ hey I am exiting healthy stage


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
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

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
    Should Contain
    ...    ${deploylog.stderr}
    ...    ${n4-exec-output}

Ensure node4 executed on-exit commands for its healthy stage
    Should Contain
    ...    ${deploylog.stderr}
    ...    ${n4-exec-healthy-output}

Ensure node3 executed on-exit commands for its create stage and this output doesn't contain any non eth0/lo interfaces
    ${extracted_text} =    Extract Text Between Markers
    ...    ${deploylog.stderr}
    ...    INFO Executed command node=node3 command="ls /sys/class/net/"

    Log    extracted node3 output is${\n}${extracted_text}    console=${True}

    Should Contain
    ...    ${extracted_text}
    ...    lo

    Should Contain
    ...    ${extracted_text}
    ...    eth0

    Should Not Contain    ${extracted_text}    eth1

Ensure node1 executed on-enter commands for its create-links stage and this output doesn't contain any non eth0/lo interfaces
    ${extracted_text} =    Extract Text Between Markers
    ...    ${deploylog.stderr}
    ...    INFO Executed command node=node1 command="ls /sys/class/net/"

    Log    ${extracted_text}    console=${True}

    Should Not Contain    ${extracted_text}    eth1

    Log    ${extracted_text}

Ensure host-exec file is created with the right content
    ${content} =    Get File    /tmp/host-exec-test
    Should Contain    ${content}    foo    msg=File does not contain the expected string

Deploy ${lab-name} lab with a single worker
    Run Keyword    Teardown

    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy --max-workers 1 -t ${CURDIR}/${lab-file}
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
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -c -t ${CURDIR}/${lab-file}

Setup
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -c -t ${CURDIR}/${lab-file}
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

Extract Text Between Markers
    # this fuckery is to get the output of the node3 exec. It captures the text between the INFO Executed command command="ls /sys/class/net/" node=node3 and the next INFO line
    [Arguments]    ${text}    ${start_marker}
    ${lines} =    String.Split To Lines    ${text}
    ${start_index} =    Set Variable    -1
    ${end_index} =    Set Variable    -1

    FOR    ${index}    ${line}    IN ENUMERATE    @{lines}
        IF    '${start_marker}' in '${line}'
            ${start_index} =    Set Variable    ${index}
            CONTINUE
        END
        ${start_index_int} =    Convert To Integer    ${start_index}
        IF    $start_index_int != -1 and 'INFO' in '${line.strip()}'
            ${end_index} =    Set Variable    ${index}
            BREAK
        END
    END

    ${extracted_text} =    Catenate    SEPARATOR=\n    @{lines}[${start_index}:${end_index}]
    RETURN    ${extracted_text}
