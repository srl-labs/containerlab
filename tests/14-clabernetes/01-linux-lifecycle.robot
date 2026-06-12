*** Settings ***
Library             OperatingSystem
Library             Process
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown
Test Tags           clabernetes    c9s


*** Variables ***
${runtime}              clabernetes
${lab-name}             c9s-linux-lifecycle
${lab-file}             01-linux-lifecycle.clab.yml
${topo}                 ${CURDIR}/${lab-file}
${client-label}         clab-node-name\=client
${events-log}           /tmp/clab-c9s-events.log
${events-err}           /tmp/clab-c9s-events.err
${recovery-timeout}     180s
${retry-interval}       5s


*** Test Cases ***
Deploy c9s linux lab
    ${output} =    Run Clab Command    deploy -t ${topo}
    Should Be Equal As Integers    ${output.rc}    0

Inspect c9s linux lab by topology and name
    ${topology_inspect} =    Run Clab Command    inspect -t ${topo}
    Should Be Equal As Integers    ${topology_inspect.rc}    0
    Should Contain    ${topology_inspect.stdout}    ${lab-name}
    Should Contain    ${topology_inspect.stdout}    client
    Should Contain    ${topology_inspect.stdout}    server

    ${name_inspect} =    Run Clab Command    inspect --name ${lab-name}
    Should Be Equal As Integers    ${name_inspect.rc}    0
    Should Contain    ${name_inspect.stdout}    ${lab-name}
    Should Contain    ${name_inspect.stdout}    client
    Should Contain    ${name_inspect.stdout}    server

Exec into client and verify dataplane
    Wait Until Keyword Succeeds    60s    ${retry-interval}    Client Eth1 Should Be Visible
    Wait Until Keyword Succeeds    60s    ${retry-interval}    Ping From Client Should Succeed

Stop server and verify dataplane interruption
    ${output} =    Run Clab Command    stop -t ${topo} --node server
    Should Be Equal As Integers    ${output.rc}    0

    Wait Until Keyword Succeeds    60s    ${retry-interval}    Ping From Client Should Fail

Start server by lab name and verify dataplane restore
    ${output} =    Run Clab Command    start --name ${lab-name} --node server
    Should Be Equal As Integers    ${output.rc}    0

    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping From Client Should Succeed

Restart server and keep dataplane working
    ${output} =    Run Clab Command    restart -t ${topo} --node server
    Should Be Equal As Integers    ${output.rc}    0

    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping From Client Should Succeed

Events command emits c9s initial state
    Remove File If Exists    ${events-log}
    Remove File If Exists    ${events-err}
    TRY
        ${cmd} =    Set Variable    ${CLAB_BIN} --runtime ${runtime} events --format json --initial-state --interface-stats=false
        Start Process    ${cmd}
        ...    shell=True
        ...    alias=c9s_events
        ...    stdout=${events-log}
        ...    stderr=${events-err}
        Sleep    5s
        Stop Events Process
        ${events} =    Get File    ${events-log}
        Log    ${events}
        Should Contain    ${events}    "type":"container"
        Should Contain    ${events}    ${lab-name}/client
        Should Contain    ${events}    ${lab-name}/server
        Validate JSON Lines    ${events-log}
    FINALLY
        Stop Events Process
        Remove File If Exists    ${events-log}
        Remove File If Exists    ${events-err}
    END

Destroy c9s linux lab
    ${output} =    Run Clab Command    destroy -t ${topo} --cleanup
    Should Be Equal As Integers    ${output.rc}    0

    ${inspect_all} =    Run Clab Command    inspect --all
    Should Not Contain    ${inspect_all.stdout}    ${lab-name}


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'clabernetes'    This suite targets the clabernetes runtime.
    Remove File If Exists    ${events-log}
    Remove File If Exists    ${events-err}
    ${output} =    Run Clab Command    destroy -t ${topo} --cleanup
    Log    Cleanup return code: ${output.rc}

Teardown
    Stop Events Process
    Run Clab Command    destroy -t ${topo} --cleanup
    Remove File If Exists    ${events-log}
    Remove File If Exists    ${events-err}

Run Clab Command
    [Arguments]    ${args}
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} ${args}
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    RETURN    ${output}

Ping From Client Should Succeed
    ${output} =    Run Clab Command
    ...    exec -t ${topo} --label ${client-label} --cmd 'ping -c 1 -W 2 10.10.10.2'
    ${combined} =    Catenate    SEPARATOR=\n    ${output.stdout}    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Match Regexp    ${combined}    (?s).*1 (packets )?received, 0% packet loss.*

Ping From Client Should Fail
    ${output} =    Run Clab Command
    ...    exec -t ${topo} --label ${client-label} --cmd 'ping -c 1 -W 2 10.10.10.2'
    ${combined} =    Catenate    SEPARATOR=\n    ${output.stdout}    ${output.stderr}
    Should Match Regexp    ${combined}    (?s).*0 (packets )?received, 100% packet loss.*

Client Eth1 Should Be Visible
    ${output} =    Run Clab Command
    ...    exec -t ${topo} --label ${client-label} --cmd 'ip link show eth1'
    ${combined} =    Catenate    SEPARATOR=\n    ${output.stdout}    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${combined}    eth1

Remove File If Exists
    [Arguments]    ${path}
    Run Keyword And Ignore Error    Remove File    ${path}

Stop Events Process
    Run Keyword And Ignore Error    Terminate Process    c9s_events    kill=True
    Run Keyword And Ignore Error    Wait For Process    c9s_events

Validate JSON Lines
    [Arguments]    ${path}
    ${contents} =    Get File    ${path}
    @{lines} =    Split To Lines    ${contents}
    FOR    ${line}    IN    @{lines}
        ${stripped} =    Strip String    ${line}
        IF    $stripped == ''    CONTINUE
        Evaluate    json.loads($stripped)    modules=json
    END
