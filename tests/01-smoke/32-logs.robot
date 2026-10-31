*** Comments ***
This suite tests the logs command with the local container runtimes.


*** Settings ***
Library             OperatingSystem
Library             Process
Library             String
Resource            ../common.robot

Suite Setup         Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
Suite Teardown      Cleanup


*** Variables ***
${lab-name}         single-node
${topo}             ${CURDIR}/01-linux-single-node.clab.yml
${runtime}          docker
${follow-log}       /tmp/clab-logs-follow.log


*** Test Cases ***
Deploy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Write markers to the node log
    Write To Node Log    clab-logs-marker-1
    Write To Node Log    clab-logs-marker-2

Logs shows the node log by topology file
    ${result} =    Run Logs    -t ${topo} -n l1
    Should Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stdout}    clab-logs-marker-1
    Should Contain    ${result.stdout}    clab-logs-marker-2

Logs tail and timestamps by lab name
    ${result} =    Run Logs    --name ${lab-name} -n l1 --tail 1 --timestamps
    Should Be Equal As Integers    ${result.rc}    0
    ${lines} =    Split To Lines    ${result.stdout}
    Length Should Be    ${lines}    1
    Should Match Regexp    ${lines}[0]    ^\\d{4}-\\d{2}-\\d{2}T\\S+ clab-logs-marker-2$

Logs follow streams new lines
    Remove File    ${follow-log}
    Start Process    ${CLAB_BIN} --runtime ${runtime} logs --name ${lab-name} -n l1 --tail 0 --follow
    ...    shell=True
    ...    alias=follow
    ...    stdout=${follow-log}
    Sleep    2s
    Write To Node Log    clab-logs-marker-3
    Wait Until Keyword Succeeds    15s    1s
    ...    File Should Contain    ${follow-log}    clab-logs-marker-3
    ${log} =    Get File    ${follow-log}
    Should Not Contain    ${log}    clab-logs-marker-1
    [Teardown]    Terminate Process    follow    kill=True

Logs rejects an unknown node
    ${result} =    Run Logs    --name ${lab-name} -n missing
    Should Not Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stderr}    missing


*** Keywords ***
Run Logs
    [Arguments]    ${args}
    ${result} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} logs ${args}
    ...    shell=True
    Log    stdout:${\n}${result.stdout}
    Log    stderr:${\n}${result.stderr}
    RETURN    ${result}

Write To Node Log
    [Arguments]    ${marker}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} exec clab-${lab-name}-l1 sh -c 'echo ${marker} > /proc/1/fd/1'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

File Should Contain
    [Arguments]    ${path}    ${text}
    ${contents} =    Get File    ${path}
    Should Contain    ${contents}    ${text}

Cleanup
    Run Keyword And Ignore Error    Terminate Process    follow    kill=True
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    Run Keyword And Ignore Error    Remove File    ${follow-log}
