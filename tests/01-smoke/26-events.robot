*** Comments ***
This suite verifies the `containerlab events` command for both plain and JSON output formats.


*** Settings ***
Library             OperatingSystem
Library             Process
Library             String
Resource            ../common.robot


*** Variables ***
${runtime}      docker
${lab-name}     2-linux-nodes
${topo}         ${CURDIR}/01-linux-nodes.clab.yml


*** Test Cases ***
Events Command Streams Plain Output
    [Documentation]    Verify that the plain formatter emits container lifecycle and interface updates enriched with netlink attributes.
    ${plain-log}    Set Variable    /tmp/clab-events-plain.log
    ${plain-err}    Set Variable    /tmp/clab-events-plain.err
    Remove File If Exists    ${plain-log}
    Remove File If Exists    ${plain-err}
    TRY
        Start Events Process    events_plain    plain    ${plain-log}    ${plain-err}
        Deploy Lab For Events
        Sleep    5s
        Destroy Lab For Events
        Sleep    3s
        Stop Events Process    events_plain
        ${plain-output} =    Get File    ${plain-log}
        Log    ${plain-output}
        Should Contain    ${plain-output}    container create
        Should Contain    ${plain-output}    container start
        Should Contain    ${plain-output}    container die
        Should Contain    ${plain-output}    interface create
        Should Contain    ${plain-output}    origin=netlink
        Should Not Contain    ${plain-output}    interface stats
    FINALLY
        Cleanup Events Scenario    events_plain
        Remove File If Exists    ${plain-log}
        Remove File If Exists    ${plain-err}
    END

Events Command Emits Interface Statistics When Enabled
    [Documentation]    Verify that enabling interface statistics augments the stream with periodic counter samples.
    ${stats-log}    Set Variable    /tmp/clab-events-stats.log
    ${stats-err}    Set Variable    /tmp/clab-events-stats.err
    Remove File If Exists    ${stats-log}
    Remove File If Exists    ${stats-err}
    TRY
        Start Events Process    events_stats    plain    ${stats-log}    ${stats-err}    False    True    2s
        Deploy Lab For Events
        Sleep    5s
        Destroy Lab For Events
        Sleep    3s
        Stop Events Process    events_stats
        ${stats-output} =    Get File    ${stats-log}
        Log    ${stats-output}
        Should Contain    ${stats-output}    container create
        Should Contain    ${stats-output}    interface stats
    FINALLY
        Cleanup Events Scenario    events_stats
        Remove File If Exists    ${stats-log}
        Remove File If Exists    ${stats-err}
    END

Events Command Emits Initial State Snapshot
    [Documentation]    Verify that enabling --initial-state emits running containers and their interfaces before live updates.
    ${snapshot-log}    Set Variable    /tmp/clab-events-snapshot.log
    ${snapshot-err}    Set Variable    /tmp/clab-events-snapshot.err
    Remove File If Exists    ${snapshot-log}
    Remove File If Exists    ${snapshot-err}
    TRY
        Deploy Lab For Events
        Sleep    3s
        Start Events Process    events_snapshot    plain    ${snapshot-log}    ${snapshot-err}    True
        Sleep    3s
        Stop Events Process    events_snapshot
        ${snapshot-output} =    Get File    ${snapshot-log}
        Log    ${snapshot-output}
        Should Contain    ${snapshot-output}    container running
        Should Contain    ${snapshot-output}    origin=snapshot
        Should Contain    ${snapshot-output}    interface snapshot
    FINALLY
        Cleanup Events Scenario    events_snapshot
        Remove File If Exists    ${snapshot-log}
        Remove File If Exists    ${snapshot-err}
    END

Events Command Streams JSON Output
    [Documentation]    Verify that JSON formatted events remain valid JSON lines and retain interface metadata.
    ${json-log}    Set Variable    /tmp/clab-events-json.log
    ${json-err}    Set Variable    /tmp/clab-events-json.err
    Remove File If Exists    ${json-log}
    Remove File If Exists    ${json-err}
    TRY
        Start Events Process    events_json    json    ${json-log}    ${json-err}
        Deploy Lab For Events
        Sleep    5s
        Destroy Lab For Events
        Sleep    3s
        Stop Events Process    events_json
        ${json-output} =    Get File    ${json-log}
        Log    ${json-output}
        Should Not Be Empty    ${json-output}
        Should Contain    ${json-output}    "type":"container"
        Should Contain    ${json-output}    "type":"interface"
        Should Contain    ${json-output}    "origin":"netlink"
        Validate JSON Lines    ${json-log}
        ${actor}    Set Variable    clab-${lab-name}-l1
        ${rc}    ${output} =    Run And Return Rc And Output
        ...    bash -lc "jq -r 'select(.actor_name==\"${actor}\") | .actor_id' ${json-log} | head -n 1"
        Log    ${output}
        Should Be Equal As Integers    ${rc}    0
        Should Not Be Empty    ${output}
    FINALLY
        Cleanup Events Scenario    events_json
        Remove File If Exists    ${json-log}
        Remove File If Exists    ${json-err}
    END


*** Keywords ***
Remove File If Exists
    [Arguments]    ${path}
    Run Keyword And Ignore Error    Remove File    ${path}

Start Events Process
    [Arguments]    ${alias}    ${format}    ${stdout}    ${stderr}    ${initial}=False    ${interface_stats}=    ${stats_interval}=
    ${cmd}    Set Variable    ${CLAB_BIN} --runtime ${runtime} events --format ${format}
    ${cmd}    Run Keyword If    '${initial}'=='True'    Catenate    ${cmd}    --initial-state    ELSE    Set Variable    ${cmd}
    ${cmd}    Run Keyword If    '${interface_stats}'=='True'    Catenate    ${cmd}    --interface-stats    ELSE IF    '${interface_stats}'=='False'    Catenate    ${cmd}    --interface-stats=false    ELSE    Set Variable    ${cmd}
    ${cmd}    Run Keyword If    '${stats_interval}'!=''    Catenate    ${cmd}    --interface-stats-interval=${stats_interval}    ELSE    Set Variable    ${cmd}
    Start Process    ${cmd}    shell=True    alias=${alias}    stdout=${stdout}    stderr=${stderr}
    Sleep    1s

Stop Events Process
    [Arguments]    ${alias}
    Run Keyword And Ignore Error    Terminate Process    ${alias}    kill=True
    Run Keyword And Ignore Error    Wait For Process    ${alias}

Deploy Lab For Events
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Destroy Lab For Events
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Cleanup Events Scenario
    [Arguments]    ${alias}
    Run Keyword And Ignore Error    Terminate Process    ${alias}    kill=True
    Run Keyword And Ignore Error    Wait For Process    ${alias}
    Run Keyword And Ignore Error    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup

Validate JSON Lines
    [Arguments]    ${path}
    ${result} =    Process.Run Process
    ...    bash -lc "python -c 'import json,sys; [json.loads(line) for line in sys.stdin]' < ${path}"
    ...    shell=True
    Log    ${result.stdout}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
