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
${applied-topo}         ${CURDIR}/01-linux-lifecycle-applied.clab.yml
${lab-namespace}        c9s-${lab-name}
${client-label}         clab-node-name\=client
${events-log}           /tmp/clab-c9s-events.log
${events-err}           /tmp/clab-c9s-events.err
${iface-events-log}     /tmp/clab-c9s-iface-events.log
${iface-events-err}     /tmp/clab-c9s-iface-events.err
${offline-env}          env CLAB_KUBECONFIG\=/nonexistent/kubeconfig KUBECONFIG\=/nonexistent/kubeconfig
${recovery-timeout}     180s
${retry-interval}       5s
${capture-file}         /tmp/clab-c9s-capture.pcapng
${chain-file}           /tmp/clab-c9s-capture-chain.pcapng
${stop-file}            /tmp/clab-c9s-capture-stop.pcapng
${stop-err}             /tmp/clab-c9s-capture-stop.err


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

Inspect interfaces lists the link interfaces with their state
    ${output} =    Run Clab Command    inspect interfaces -t ${topo} -f json
    Should Be Equal As Integers    ${output.rc}    0
    ${containers} =    Evaluate    json.loads($output.stdout)    modules=json
    Length Should Be    ${containers}    2
    FOR    ${container}    IN    @{containers}
        Should Match Regexp    ${container}[name]    ^${lab-name}-(client|server)$
        Length Should Be    ${container}[interfaces]    1
        ${iface} =    Set Variable    ${container}[interfaces][0]
        Should Be Equal    ${iface}[name]    eth1
        Should Be Equal    ${iface}[type]    veth
        Should Be Equal    ${iface}[state]    up
        Should Match Regexp    ${iface}[mac]    ^([0-9a-f]{2}:){5}[0-9a-f]{2}$
        Should Be True    ${iface}[ifindex] > 0 and ${iface}[mtu] > 0
    END

    ${one} =    Run Clab Command    inspect interfaces --name ${lab-name} -n client -f json
    Should Be Equal As Integers    ${one.rc}    0
    ${only} =    Evaluate    [c["name"] for c in json.loads($one.stdout)]    modules=json
    Should Be Equal    ${only}    ${{["${lab-name}-client"]}}

Capture lists the capturable interfaces with their peers
    ${output} =    Run Clab Command    capture -t ${topo} --list -f json
    Should Be Equal As Integers    ${output.rc}    0
    ${interfaces} =    Evaluate    json.loads($output.stdout)    modules=json
    ${want} =    Evaluate
    ...    [{"node": "client", "interface": "eth1", "peer_node": "server", "peer_interface": "eth1"}, {"node": "server", "interface": "eth1", "peer_node": "client", "peer_interface": "eth1"}]
    Should Be Equal    ${interfaces}    ${want}

Capture both ends of a link into one pcapng stream
    Skip If    not shutil.which("tshark")    tshark is not installed
    Remove File If Exists    ${capture-file}
    Start Process    ${CLAB_BIN} --runtime ${runtime} exec -t ${topo} --label ${client-label} --cmd 'ping -c 10 -i 0.3 10.10.10.2'
    ...    shell=True    alias=c9s_ping
    ${output} =    Run Clab Command
    ...    capture -t ${topo} --target client:eth1 --target server:eth1 --duration 5s -w ${capture-file}
    Wait For Process    c9s_ping    timeout=30s
    Should Be Equal As Integers    ${output.rc}    0

    ${idbs} =    Process.Run Process    capinfos -T -m -I ${capture-file}    shell=True
    Log    ${idbs.stdout}
    ${names} =    Process.Run Process
    ...    tshark -r ${capture-file} -T fields -e frame.interface_name -Y icmp | sort | uniq -c
    ...    shell=True
    Log    ${names.stdout}
    Should Be Equal As Integers    ${names.rc}    0
    ${counts} =    Evaluate
    ...    {l.split()[1]: int(l.split()[0]) for l in $names.stdout.splitlines() if l.strip()}
    Should Be Equal    ${{sorted($counts)}}    ${{["client:eth1", "server:eth1"]}}
    Should Be True    ${counts}[client:eth1] >= 10 and ${counts}[server:eth1] >= 10

    ${info} =    Process.Run Process    capinfos ${capture-file}    shell=True
    Log    ${info.stdout}
    Should Contain    ${info.stdout}    Number of interfaces in file: 2
    Should Contain    ${info.stdout}    Capture application: containerlab
    Should Contain    ${info.stdout}    nanoseconds

    # the packets of both nodes are merged in timestamp order
    ${times} =    Process.Run Process
    ...    tshark -r ${capture-file} -T fields -e frame.time_epoch
    ...    shell=True
    Should Be Equal As Integers    ${times.rc}    0
    ${epochs} =    Evaluate    [decimal.Decimal(x) for x in $times.stdout.split()]    modules=decimal
    ${count} =    Get Length    ${epochs}
    Should Be True    ${count} >= 20    only ${count} packets
    ${inversions} =    Evaluate
    ...    [(i, str($epochs[i - 1]), str($epochs[i])) for i in range(1, len($epochs)) if $epochs[i] < $epochs[i - 1]]
    Should Be Empty    ${inversions}    frame.time_epoch decreases
    [Teardown]    Remove File If Exists    ${capture-file}

Stopping a capture leaves no capture process in clabwire
    Skip If    not shutil.which("docker")    docker is needed to list the processes of a KinD worker
    Skip If    not shutil.which("capinfos")    capinfos is not installed
    Remove File If Exists    ${stop-file}
    Start Process    ${CLAB_BIN}    --runtime    ${runtime}    capture    -t    ${topo}
    ...    --target    client:eth1    --target    server:eth1    -w    ${stop-file}
    ...    alias=c9s_capture    stderr=${stop-err}
    Wait Until Keyword Succeeds    60s    1s    Capture Process Should Run In Clabwire    client
    Wait Until Keyword Succeeds    10s    1s    Capture Process Should Run In Clabwire    server

    Send Signal To Process    SIGINT    c9s_capture
    ${result} =    Wait For Process    c9s_capture    timeout=30s    on_timeout=kill
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    # Kubernetes does not stop an exec'd process when its stream closes; clabwire ends the
    # capture when containerlab closes its stdin
    Wait Until Keyword Succeeds    5s    500ms    No Capture Process Should Run In Clabwire    client
    Wait Until Keyword Succeeds    5s    500ms    No Capture Process Should Run In Clabwire    server

    ${info} =    Process.Run Process    capinfos ${stop-file}    shell=True
    Should Be Equal As Integers    ${info.rc}    0
    Should Contain    ${info.stdout}    Number of interfaces in file: 2
    [Teardown]    Run Keywords    Remove File If Exists    ${stop-file}
    ...    AND    Remove File If Exists    ${stop-err}

Capture chains segments without duplicates or gaps
    Skip If    not shutil.which("tshark")    tshark is not installed
    Remove File If Exists    ${chain-file}
    Start Process    ${CLAB_BIN} --runtime ${runtime} exec -t ${topo} --label ${client-label} --cmd 'ping -c 130 -i 0.2 10.10.10.2'
    ...    shell=True    alias=c9s_ping
    Sleep    1s
    # a 6 s segment length forces four switches in 20 s
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} --log-level debug capture -t ${topo} --target client:eth1 --duration 20s -w ${chain-file}
    ...    shell=True
    ...    env:CLAB_C9S_CAPTURE_SEGMENT=6s
    Log    ${output.stderr}
    Wait For Process    c9s_ping    timeout=60s
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stderr}    segment=4
    Should Not Contain    ${output.stderr}    gap

    ${seqs} =    Process.Run Process
    ...    tshark -r ${chain-file} -Y 'icmp.type \=\= 8' -T fields -e icmp.seq
    ...    shell=True
    Should Be Equal As Integers    ${seqs.rc}    0
    ${numbers} =    Evaluate    [int(x) for x in $seqs.stdout.split()]
    ${count} =    Get Length    ${numbers}
    Should Be True    ${count} >= 90    only ${count} echo requests
    ${unique} =    Evaluate    len(set($numbers))
    Should Be Equal As Integers    ${unique}    ${count}    duplicate echo requests
    ${span} =    Evaluate    max($numbers) - min($numbers) + 1
    Should Be Equal As Integers    ${span}    ${count}    missing echo requests
    [Teardown]    Remove File If Exists    ${chain-file}

Capture works for a lab editor and is forbidden without a binding
    ${rbac} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} create serviceaccount capture-editor && kubectl -n ${lab-namespace} create rolebinding capture-editor --clusterrole\=clabernetes-lab-editor --serviceaccount\=${lab-namespace}:capture-editor
    ...    shell=True
    Log    ${rbac.stdout}${\n}${rbac.stderr}
    Should Be Equal As Integers    ${rbac.rc}    0

    ${result} =    Process.Run Process
    ...    go test ./core/ -run TestC9sClusterCapture -count\=1 -v
    ...    shell=True
    ...    cwd=${CURDIR}/../..
    ...    env:CLAB_C9S_TEST_LAB=${lab-name}
    ...    env:CLAB_C9S_TEST_CAPTURE_USER=system:serviceaccount:${lab-namespace}:capture-editor
    ...    env:CLAB_C9S_TEST_FORBIDDEN_USER=system:serviceaccount:${lab-namespace}:nobody
    Log    ${result.stdout}${\n}${result.stderr}    console=${True}
    Should Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stdout}    --- PASS: TestC9sClusterCapture/capture_both_ends_of_a_link
    Should Contain    ${result.stdout}    --- PASS: TestC9sClusterCapture/restart_as_lab_editor
    Should Contain    ${result.stdout}    --- PASS: TestC9sClusterCapture/forbidden_identity
    Should Not Contain    ${result.stdout}    SKIP
    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping From Client Should Succeed

Logs shows the device container log
    ${result} =    Run Clab Command    logs -t ${topo} -n client --tail 2 --timestamps
    Should Be Equal As Integers    ${result.rc}    0
    ${lines} =    Split To Lines    ${result.stdout}
    ${count} =    Get Length    ${lines}
    Should Be True    0 < ${count} <= 2
    Should Match Regexp    ${lines}[0]    ^\\d{4}-\\d{2}-\\d{2}T\\S+\\s

    # the node name is accepted as the topology spells it or as Kubernetes carries it
    ${by_name} =    Run Clab Command    logs --name ${lab-name} -n CLIENT --tail 1
    Should Be Equal As Integers    ${by_name.rc}    0

    ${missing} =    Run Clab Command    logs --name ${lab-name} -n missing
    Should Not Be Equal As Integers    ${missing.rc}    0
    Should Contain    ${missing.stderr}    missing

Logs follow streams new device log lines
    Remove File If Exists    ${events-log}
    TRY
        Start Process    ${CLAB_BIN} --runtime ${runtime} logs --name ${lab-name} -n client --tail 0 --follow
        ...    shell=True
        ...    alias=c9s_logs
        ...    stdout=${events-log}
        ...    stderr=${events-err}
        Sleep    3s
        # nginx in the multitool image logs every request
        Run Clab Command    exec -t ${topo} --label ${client-label} --cmd 'curl -s -o /dev/null http://127.0.0.1/clab-follow-marker'
        Wait Until Keyword Succeeds    30s    1s    Log File Should Contain    ${events-log}    clab-follow-marker
    FINALLY
        Run Keyword And Ignore Error    Terminate Process    c9s_logs    kill=True
        Remove File If Exists    ${events-log}
        Remove File If Exists    ${events-err}
    END

Library attaches, port-forwards, streams logs and samples stats as an impersonated user
    ${rbac} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} create role c9s-lab-reader --verb\=get,list,watch --resource\=topologies.c9s.run,nodes.c9s.run,links.c9s.run,nodeprofiles.c9s.run,pods.metrics.k8s.io && kubectl -n ${lab-namespace} create rolebinding clab-alice-reader --role\=c9s-lab-reader --user\=clab:alice && kubectl -n ${lab-namespace} create rolebinding clab-alice-admin --clusterrole\=admin --user\=clab:alice
    ...    shell=True
    Log    ${rbac.stdout}${\n}${rbac.stderr}
    Should Be Equal As Integers    ${rbac.rc}    0

    ${result} =    Process.Run Process
    ...    go test ./core/ -run TestC9sClusterNodeAccess -count\=1 -v
    ...    shell=True
    ...    cwd=${CURDIR}/../..
    ...    env:CLAB_C9S_TEST_LAB=${lab-name}
    ...    env:CLAB_C9S_TEST_USER=clab:alice
    ...    env:CLAB_C9S_TEST_FORBIDDEN_USER=clab:mallory
    Log    ${result.stdout}${\n}${result.stderr}    console=${True}
    Should Be Equal As Integers    ${result.rc}    0
    Should Contain    ${result.stdout}    --- PASS: TestC9sClusterNodeAccess/attach_with_a_terminal
    Should Contain    ${result.stdout}    --- PASS: TestC9sClusterNodeAccess/port-forward
    Should Contain    ${result.stdout}    --- PASS: TestC9sClusterNodeAccess/stats
    Should Contain    ${result.stdout}    --- PASS: TestC9sClusterNodeAccess/forbidden_identity
    Should Not Contain    ${result.stdout}    SKIP

Stop server and verify dataplane interruption
    ${output} =    Run Clab Command    stop -t ${topo} --node server
    Should Be Equal As Integers    ${output.rc}    0

    # stop is a run state patch on the Node; the controller removes the Pod
    ${run_state} =    Node Field    server    {.spec.runState}
    Should Be Equal    ${run_state}    Stopped
    ${stopped} =    Node Field    server    {.status.conditions[?(@.type=="Stopped")].status}
    Should Be Equal    ${stopped}    True
    ${pods} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get pods -l c9s.run/topologyNode\=server -o name
    ...    shell=True
    Should Be Empty    ${pods.stdout}
    ${inspect} =    Run Clab Command    inspect -t ${topo} --format json
    Should Contain    ${inspect.stdout}    "state": "stopped"

    Wait Until Keyword Succeeds    60s    ${retry-interval}    Ping From Client Should Fail

Apply while server is stopped keeps it stopped
    ${output} =    Run Clab Command    deploy -t ${applied-topo}
    Should Be Equal As Integers    ${output.rc}    0
    # the wait settles on the stopped node instead of waiting for it
    Should Contain    ${output.stderr}    C9s lab is ready apart from stopped nodes
    Should Contain    ${output.stderr}    server (stopped)

    ${run_state} =    Node Field    server    {.spec.runState}
    Should Be Equal    ${run_state}    Stopped
    ${env} =    Node Field    server    {.spec.env.CLAB_APPLY_MARK}
    Should Be Equal    ${env}    applied-while-stopped

Start server by lab name and verify dataplane restore
    ${output} =    Run Clab Command    start --name ${lab-name} --node server
    Should Be Equal As Integers    ${output.rc}    0

    ${run_state} =    Node Field    server    {.spec.runState}
    Should Be Equal    ${run_state}    Running
    ${readiness} =    Node Field    server    {.status.readiness}
    Should Be Equal    ${readiness}    ready

    # the server boots the definition applied while it was stopped
    ${mark} =    Run Clab Command
    ...    exec -t ${applied-topo} --label clab-node-name\=server --cmd 'printenv CLAB_APPLY_MARK'
    ${mark_output} =    Catenate    SEPARATOR=\n    ${mark.stdout}    ${mark.stderr}
    Should Be Equal As Integers    ${mark.rc}    0
    Should Contain    ${mark_output}    applied-while-stopped

    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping From Client Should Succeed

Restart server and keep dataplane working
    ${before} =    Server Pod Name
    ${output} =    Run Clab Command    restart -t ${topo} --node server
    Should Be Equal As Integers    ${output.rc}    0

    ${after} =    Server Pod Name
    Should Not Be Equal    ${before}    ${after}
    ${token} =    Node Field    server    {.metadata.annotations.c9s\\.run/restart-requested}
    ${observed} =    Node Field    server    {.status.observedRestartToken}
    Should Not Be Empty    ${token}
    Should Be Equal    ${token}    ${observed}

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

Apply without a Topology resource keeps a stopped node stopped
    ${output} =    Run Clab Command    stop -t ${topo} --node client
    Should Be Equal As Integers    ${output.rc}    0

    ${plan} =    Run Clab Command    deploy -t ${applied-topo} --no-topology-cr --dry-run --format json
    Should Be Equal As Integers    ${plan.rc}    0
    ${changes} =    Evaluate    [c["kind"] + "/" + c["name"] for c in json.loads($plan.stdout)["changes"]]    modules=json
    Should Contain    ${changes}    Node/server
    Should Not Contain    ${changes}    Node/client

    ${output} =    Run Clab Command    deploy -t ${applied-topo} --no-topology-cr
    Should Be Equal As Integers    ${output.rc}    0
    ${run_state} =    Node Field    client    {.spec.runState}
    Should Be Equal    ${run_state}    Stopped

    ${output} =    Run Clab Command    start -t ${topo} --node client
    Should Be Equal As Integers    ${output.rc}    0
    Wait Until Keyword Succeeds    ${recovery-timeout}    ${retry-interval}    Ping From Client Should Succeed

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

Clabwire Capture Processes
    [Arguments]    ${node}
    ${result} =    Process.Run Process
    ...    ${CURDIR}/clabwire-captures.sh ${lab-namespace} ${node}
    ...    shell=True
    Log    ${result.stdout}${\n}${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    ${lines} =    Get Lines Containing String    ${result.stdout}    packet-capture
    ${count} =    Get Line Count    ${lines}
    RETURN    ${count}

Capture Process Should Run In Clabwire
    [Arguments]    ${node}
    ${count} =    Clabwire Capture Processes    ${node}
    Should Be True    ${count} > 0    no packet-capture process in the clabwire container of ${node}

No Capture Process Should Run In Clabwire
    [Arguments]    ${node}
    ${count} =    Clabwire Capture Processes    ${node}
    Should Be Equal As Integers    ${count}    0
    ...    ${count} packet-capture processes still run in the clabwire container of ${node}

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

Log File Should Contain
    [Arguments]    ${path}    ${text}
    ${contents} =    Get File    ${path}
    Should Contain    ${contents}    ${text}

Persistent Claim Volumes
    ${claims} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get pvc -l c9s.run/topologyOwner\=${lab-name} -o jsonpath\='{range .items[*]}{.metadata.name}\={.spec.volumeName}{" "}{end}'
    ...    shell=True
    Should Be Equal As Integers    ${claims.rc}    0
    ${volumes} =    Strip String    ${claims.stdout}
    RETURN    ${volumes}

Node Field
    [Arguments]    ${node}    ${jsonpath}
    ${result} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get nodes.c9s.run ${node} -o jsonpath\='${jsonpath}'
    ...    shell=True
    Should Be Equal As Integers    ${result.rc}    0    ${result.stderr}
    RETURN    ${result.stdout}

Server Pod Name
    ${result} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get pods -l c9s.run/topologyNode\=server --field-selector status.phase\=Running -o jsonpath\='{.items[*].metadata.name}'
    ...    shell=True
    Should Be Equal As Integers    ${result.rc}    0
    RETURN    ${result.stdout}

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
