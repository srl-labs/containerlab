*** Settings ***
Library             OperatingSystem
Library             Process
Library             String
Library             Collections
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown
Test Tags           clabernetes    c9s


*** Variables ***
${runtime}              c9s
${lab-name}             c9s-linux-lifecycle
${lab-file}             01-linux-lifecycle.clab.yml
${topo}                 ${CURDIR}/${lab-file}
${lab-namespace}        c9s-${lab-name}
${client-label}         clab-node-name\=client
${events-log}           /tmp/clab-c9s-events.log
${events-err}           /tmp/clab-c9s-events.err
${iface-events-log}     /tmp/clab-c9s-iface-events.log
${iface-events-err}     /tmp/clab-c9s-iface-events.err
${offline-env}          env CLAB_KUBECONFIG\=/nonexistent/kubeconfig KUBECONFIG\=/nonexistent/kubeconfig
${recovery-timeout}     180s
${retry-interval}       5s


*** Test Cases ***
Deploy c9s linux lab
    ${output} =    Run Clab Command    deploy -t ${topo}
    Should Be Equal As Integers    ${output.rc}    0

Deploy creates the Topology resource and controller-compiled primitives
    ${topology} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get topology.c9s.run ${lab-name} -o name
    ...    shell=True
    Should Be Equal As Integers    ${topology.rc}    0
    Should Contain    ${topology.stdout}    ${lab-name}

    ${nodes} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get nodes.c9s.run -l c9s.run/topologyOwner\=${lab-name} -o name
    ...    shell=True
    Should Be Equal As Integers    ${nodes.rc}    0
    Should Contain    ${nodes.stdout}    node.c9s.run/client
    Should Contain    ${nodes.stdout}    node.c9s.run/server

    ${links} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get links.c9s.run -l c9s.run/topologyOwner\=${lab-name} -o name
    ...    shell=True
    Should Be Equal As Integers    ${links.rc}    0
    Should Not Be Empty    ${links.stdout}

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

Inspect JSON locates the c9s workload of every node
    ${output} =    Run Clab Command    inspect -t ${topo} --format json
    Should Be Equal As Integers    ${output.rc}    0
    ${labs} =    Evaluate    json.loads($output.stdout)    modules=json
    ${nodes} =    Get From Dictionary    ${labs}    ${lab-name}
    Length Should Be    ${nodes}    2
    FOR    ${node}    IN    @{nodes}
        ${short} =    Fetch From Right    ${node}[name]    ${lab-name}-
        Should Be Equal    ${node}[container_id]    ${lab-namespace}/${lab-name}/${short}
        Should Be Equal    ${node}[namespace]    ${lab-namespace}

        ${pod} =    Process.Run Process
        ...    kubectl -n ${lab-namespace} get pod ${node}[pod] -o jsonpath\='{.metadata.annotations.kubectl\\.kubernetes\\.io/default-container}'
        ...    shell=True
        Should Be Equal As Integers    ${pod.rc}    0
        Should Be Equal    ${pod.stdout}    ${node}[container]

        ${uid} =    Process.Run Process
        ...    kubectl -n ${lab-namespace} get nodes.c9s.run ${short} -o jsonpath\='{.metadata.uid}'
        ...    shell=True
        Should Be Equal As Integers    ${uid.rc}    0
        Should Be Equal    ${uid.stdout}    ${node}[node_uid]

        ${ports} =    Evaluate    [p["port"] for p in $node["exposed_ports"]]
        Should Contain    ${ports}    ${22}
    END

Inspect all is scoped by the namespace override
    ${scoped} =    Run Clab Command    --namespace ${lab-namespace} inspect --all --format json
    Should Be Equal As Integers    ${scoped.rc}    0
    Should Contain    ${scoped.stdout}    ${lab-name}

    ${other} =    Run Clab Command    --namespace default inspect --all --format json
    Should Be Equal As Integers    ${other.rc}    0
    Should Not Contain    ${other.stdout}    ${lab-name}

Validate and emit-crs work without a kubeconfig
    ${validate} =    Run Offline Clab Command    validate -t ${topo}
    Should Be Equal As Integers    ${validate.rc}    0
    Should Contain    ${validate.stderr}    Topology is valid for lab runtime

    ${emit} =    Run Offline Clab Command    deploy -t ${topo} --emit-crs
    Should Be Equal As Integers    ${emit.rc}    0
    Should Contain    ${emit.stdout}    kind: Topology

Graph renders from the topology file without a cluster
    ${mermaid} =    Run Offline Clab Command    graph -t ${topo} --mermaid
    Should Be Equal As Integers    ${mermaid.rc}    0
    Should Contain    ${mermaid.stderr}    Created mermaid diagram file
    File Should Exist    ${CURDIR}/clab-${lab-name}/graph/01-linux-lifecycle.clab.mermaid
    ${diagram} =    Get File    ${CURDIR}/clab-${lab-name}/graph/01-linux-lifecycle.clab.mermaid
    Should Contain    ${diagram}    client
    Should Contain    ${diagram}    server

    ${dot} =    Run Offline Clab Command    graph -t ${topo} --dot
    Should Be Equal As Integers    ${dot.rc}    0
    File Should Exist    ${CURDIR}/clab-${lab-name}/graph/01-linux-lifecycle.clab.dot

    ${server} =    Run Offline Clab Command    graph -t ${topo}
    Should Not Be Equal As Integers    ${server.rc}    0
    Should Contain    ${server.stderr}    --offline
    [Teardown]    Remove Directory    ${CURDIR}/clab-${lab-name}    recursive=True

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

Events are scoped to the lab and the namespace
    ${lab_events} =    Collect Events    --name ${lab-name} events --format json --initial-state
    Should Contain    ${lab_events}    "actor_id":"${lab-namespace}/${lab-name}/client"
    Should Contain    ${lab_events}    "type":"interface"

    ${ns_events} =    Collect Events    --namespace ${lab-namespace} events --format json --initial-state
    Should Contain    ${ns_events}    "actor_id":"${lab-namespace}/${lab-name}/client"

    ${other_events} =    Collect Events    --namespace default events --format json --initial-state
    Should Not Contain    ${other_events}    ${lab-name}

Events report interface up and down
    Remove File If Exists    ${iface-events-log}
    TRY
        Start Process    ${CLAB_BIN} --runtime ${runtime} --name ${lab-name} events --format json
        ...    shell=True
        ...    alias=c9s_events
        ...    stdout=${iface-events-log}
        ...    stderr=${iface-events-err}
        Sleep    5s
        Set Client Eth1    down
        Wait Until Keyword Succeeds    30s    1s
        ...    Interface Event Should Be Logged    ${iface-events-log}    client    eth1    down
        # the peer loses carrier
        Wait Until Keyword Succeeds    30s    1s
        ...    Interface Event Should Be Logged    ${iface-events-log}    server    eth1    down
        Set Client Eth1    up
        Wait Until Keyword Succeeds    30s    1s
        ...    Interface Event Should Be Logged    ${iface-events-log}    client    eth1    up
        Validate JSON Lines    ${iface-events-log}
    FINALLY
        Stop Events Process
        Set Client Eth1    up
        Remove File If Exists    ${iface-events-log}
        Remove File If Exists    ${iface-events-err}
    END

Destroy without cleanup keeps the persistent claims
    ${before} =    Persistent Claim Volumes
    Should Not Be Empty    ${before}

    ${output} =    Run Clab Command    destroy -t ${topo}
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stderr}    Keeping persistent node storage

    ${after} =    Persistent Claim Volumes
    Should Be Equal    ${after}    ${before}
    Set Suite Variable    ${retained-volumes}    ${before}

Deploy reuses the retained persistent claims
    ${output} =    Run Clab Command    deploy -t ${topo}
    Should Be Equal As Integers    ${output.rc}    0

    ${volumes} =    Persistent Claim Volumes
    Should Be Equal    ${volumes}    ${retained-volumes}

Destroy c9s linux lab
    ${output} =    Run Clab Command    destroy -t ${topo} --cleanup
    Should Be Equal As Integers    ${output.rc}    0

    ${inspect_all} =    Run Clab Command    inspect --all
    Should Not Contain    ${inspect_all.stdout}    ${lab-name}

    ${namespace} =    Process.Run Process
    ...    kubectl get namespace ${lab-namespace} --ignore-not-found -o name
    ...    shell=True
    Should Be Equal As Integers    ${namespace.rc}    0
    Should Be Empty    ${namespace.stdout}

Deploy c9s linux lab without a Topology resource
    ${output} =    Run Clab Command    deploy -t ${topo} --no-topology-cr
    Should Be Equal As Integers    ${output.rc}    0

    ${topology} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get topology.c9s.run ${lab-name} --ignore-not-found -o name
    ...    shell=True
    Should Be Equal As Integers    ${topology.rc}    0
    Should Be Empty    ${topology.stdout}

    ${nodes} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get nodes.c9s.run -l c9s.run/topologyOwner\=${lab-name} -o name
    ...    shell=True
    Should Be Equal As Integers    ${nodes.rc}    0
    Should Contain    ${nodes.stdout}    node.c9s.run/client
    Should Contain    ${nodes.stdout}    node.c9s.run/server

Destroy c9s linux lab deployed without a Topology resource
    ${output} =    Run Clab Command    destroy -t ${topo} --cleanup
    Should Be Equal As Integers    ${output.rc}    0

    ${namespace} =    Process.Run Process
    ...    kubectl get namespace ${lab-namespace} --ignore-not-found -o name
    ...    shell=True
    Should Be Equal As Integers    ${namespace.rc}    0
    Should Be Empty    ${namespace.stdout}


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'c9s'    This suite targets the c9s runtime.
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

Run Offline Clab Command
    [Arguments]    ${args}
    ${output} =    Process.Run Process
    ...    ${offline-env} ${CLAB_BIN} --runtime ${runtime} ${args}
    ...    shell=True
    ...    cwd=${CURDIR}
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    RETURN    ${output}

Collect Events
    [Arguments]    ${args}
    Remove File If Exists    ${events-log}
    TRY
        Start Process    ${CLAB_BIN} --runtime ${runtime} ${args}
        ...    shell=True
        ...    alias=c9s_events
        ...    stdout=${events-log}
        ...    stderr=${events-err}
        Sleep    6s
    FINALLY
        Stop Events Process
    END
    ${events} =    Get File    ${events-log}
    Log    ${events}
    Validate JSON Lines    ${events-log}
    Remove File If Exists    ${events-log}
    Remove File If Exists    ${events-err}
    RETURN    ${events}

Set Client Eth1
    [Arguments]    ${state}
    Run Clab Command    exec -t ${topo} --label ${client-label} --cmd 'ip link set dev eth1 ${state}'

Interface Event Should Be Logged
    [Arguments]    ${path}    ${node}    ${ifname}    ${action}
    ${contents} =    Get File    ${path}
    ${actor} =    Set Variable    ${lab-namespace}/${lab-name}/${node}
    ${found} =    Evaluate
    ...    ($action, $actor, $ifname) in [(e.get("action"), e.get("actor_id"), e.get("attributes", {}).get("ifname")) for e in map(json.loads, filter(str.strip, $contents.splitlines())) if e.get("type") == "interface"]
    ...    modules=json
    Should Be True    ${found}    no interface ${action} event for ${node}:${ifname}

Persistent Claim Volumes
    ${claims} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get pvc -l c9s.run/topologyOwner\=${lab-name} -o jsonpath\='{range .items[*]}{.metadata.name}\={.spec.volumeName}{" "}{end}'
    ...    shell=True
    Should Be Equal As Integers    ${claims.rc}    0
    ${volumes} =    Strip String    ${claims.stdout}
    RETURN    ${volumes}

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
