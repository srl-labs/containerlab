*** Comments ***
This suite tests link impairments with the c9s runtime: the topology's impairments become the
netem of the c9s Links and clabwire programs them on the node interfaces, and tools netem changes
the Links of a lab deployed with --no-topology-cr.


*** Settings ***
Library             OperatingSystem
Library             Process
Library             String
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Run Clab Command    destroy -t ${topo} --cleanup
Test Tags           clabernetes    c9s


*** Variables ***
${runtime}          c9s
${lab-name}         c9s-link-netem
${topo}             ${CURDIR}/03-link-netem.clab.yml
${lab-namespace}    c9s-${lab-name}
${client-label}     clab-node-name\=client


*** Test Cases ***
Deploy c9s lab with link impairments
    ${output} =    Run Clab Command    deploy -t ${topo}
    Should Be Equal As Integers    ${output.rc}    0

The Link declares the topology's impairment
    ${links} =    Process.Run Process
    ...    kubectl -n ${lab-namespace} get links.c9s.run -l c9s.run/topologyOwner\=${lab-name} -o jsonpath\='{.items[*].spec.endpointA.netem.delay}'
    ...    shell=True
    Should Be Equal As Integers    ${links.rc}    0
    Should Be Equal    ${links.stdout}    50ms

Clabwire programs the impairment on the node interface
    Wait Until Keyword Succeeds    120s    5s    Client Qdisc Should Contain    delay 50ms

Tools netem shows the impairment
    ${output} =    Run Clab Command    tools netem show -t ${topo} -n client -f json
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stdout}    "delay": "50ms"

Tools netem leaves a Topology-owned lab to the topology
    ${output} =    Run Clab Command    tools netem set -t ${topo} -n client -i eth1 --delay 200ms
    Should Not Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stderr}    --no-topology-cr

Deploy the lab without a Topology resource
    ${output} =    Run Clab Command    destroy -t ${topo} --cleanup
    Should Be Equal As Integers    ${output.rc}    0
    ${output} =    Run Clab Command    deploy -t ${topo} --no-topology-cr
    Should Be Equal As Integers    ${output.rc}    0
    Wait Until Keyword Succeeds    120s    5s    Client Qdisc Should Contain    delay 50ms

Tools netem changes the impairment live
    ${output} =    Run Clab Command    tools netem set -t ${topo} -n client -i eth1 --delay 200ms --loss 1
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stdout}    200ms
    Wait Until Keyword Succeeds    180s    5s    Client Qdisc Should Contain    delay 200ms

Tools netem resets the impairment
    ${output} =    Run Clab Command    tools netem reset -t ${topo} -n client -i eth1
    Should Be Equal As Integers    ${output.rc}    0
    Wait Until Keyword Succeeds    180s    5s    Client Qdisc Should Contain    noqueue

A deploy restores the topology's impairments
    ${output} =    Run Clab Command    deploy -t ${topo} --no-topology-cr
    Should Be Equal As Integers    ${output.rc}    0
    Wait Until Keyword Succeeds    180s    5s    Client Qdisc Should Contain    delay 50ms


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'c9s'    This suite targets the c9s runtime.
    ${output} =    Run Clab Command    destroy -t ${topo} --cleanup
    Log    Cleanup return code: ${output.rc}

Run Clab Command
    [Arguments]    ${args}
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} ${args}
    ...    shell=True
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    RETURN    ${output}

Client Qdisc Should Contain
    [Arguments]    ${expected}
    ${output} =    Run Clab Command
    ...    exec -t ${topo} --label ${client-label} --cmd 'tc qdisc show dev eth1'
    Should Be Equal As Integers    ${output.rc}    0
    Should Contain    ${output.stdout}${output.stderr}    ${expected}
