*** Settings ***
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown


*** Variables ***
${lab-file}         27-node-lifecycle.clab.yml
${lab-name}         node-lifecycle
${runtime}          docker
${l1-label}         clab-node-name\=l1
${recovery-timeout}    120s
${retry-interval}      2s


*** Test Cases ***
Deploy lifecycle lab
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    ...    shell=True

    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

    Should Be Equal As Integers    ${output.rc}    0

Dataplane ping succeeds before lifecycle operations
    Wait Until Keyword Succeeds    30s    ${retry-interval}    Ping Over Dataplane Succeeds

Stop l2 and verify dataplane interruption
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} stop -t ${CURDIR}/${lab-file} --node l2
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    Should Be Equal As Integers    ${output.rc}    0

    Wait Until Keyword Succeeds    30s    ${retry-interval}    Ping Over Dataplane Fails

Start l2 by lab name and verify dataplane restore
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} start --name ${lab-name} --node l2
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    Should Be Equal As Integers    ${output.rc}    0

    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping Over Dataplane Succeeds

Restart l2 and keep dataplane working
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} restart -t ${CURDIR}/${lab-file} --node l2
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    Should Be Equal As Integers    ${output.rc}    0

    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping Over Dataplane Succeeds

Stop all nodes when node flag is omitted
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} stop -t ${CURDIR}/${lab-file}
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

    Should Be Equal As Integers    ${output.rc}    0
    Node Should Be In State    l1    exited
    Node Should Be In State    l2    exited

Start all nodes when node flag is omitted
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} start --name ${lab-name}
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

    Should Be Equal As Integers    ${output.rc}    0
    Node Should Be In State    l1    running
    Node Should Be In State    l2    running
    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping Over Dataplane Succeeds

Restart all nodes when node flag is omitted
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} restart -t ${CURDIR}/${lab-file}
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

    Should Be Equal As Integers    ${output.rc}    0
    Node Should Be In State    l1    running
    Node Should Be In State    l2    running
    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping Over Dataplane Succeeds

Lifecycle command fails without topology-or-name input
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} start --node l2
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

    Should Not Be Equal As Integers    ${output.rc}    0
    ${combined} =    Catenate    SEPARATOR=\n    ${output.stdout}    ${output.stderr}
    Should Contain    ${combined}    No topology files matching the pattern *.clab.yml or *.clab.yaml found.


*** Keywords ***
Setup
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

Teardown
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}

Ping Over Dataplane Succeeds
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label ${l1-label} --cmd 'ping -c 1 -W 1 10.10.10.2'
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    ${combined} =    Catenate    SEPARATOR=\n    ${output.stdout}    ${output.stderr}
    Should Match Regexp    ${combined}    (?m)^.*1 packets transmitted, 1 packets received, 0% packet loss$

Ping Over Dataplane Fails
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label ${l1-label} --cmd 'ping -c 1 -W 1 10.10.10.2'
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    ${combined} =    Catenate    SEPARATOR=\n    ${output.stdout}    ${output.stderr}
    Should Match Regexp    ${combined}    (?m)^.*1 packets transmitted, 0 packets received, 100% packet loss$

Node Should Be In State
    [Arguments]    ${node}    ${state}
    ${output} =    Process.Run Process
    ...    ${runtime} inspect -f '{{.State.Status}}' clab-${lab-name}-${node}
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    Should Be Equal As Integers    ${output.rc}    0
    Should Match Regexp    ${output.stdout}    (?im)^${state}\\s*$
