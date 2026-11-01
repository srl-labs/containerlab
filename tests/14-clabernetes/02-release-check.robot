*** Comments ***
The c9s runtime refuses clusters whose clabernetes release lacks Node run state and the clabwire
capture contract. This suite needs a second cluster that serves the older c9s.run CRDs (for
example a KinD cluster with only the clabernetes 0.10.1 CRDs applied); set
CLAB_C9S_OLD_KUBECONFIG to its kubeconfig. Without it, the suite is skipped.


*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Test Tags           clabernetes    c9s


*** Variables ***
${runtime}          c9s
${topo}             ${CURDIR}/01-linux-lifecycle.clab.yml
${lab-namespace}    c9s-c9s-linux-lifecycle
${required}         clabernetes with Node run state and the clabwire capture contract (0.11 or newer)


*** Test Cases ***
Cluster commands fail before the first cluster call
    FOR    ${args}    IN
    ...    inspect --all
    ...    deploy -t ${topo}
    ...    stop -t ${topo} -n server
    ...    capture -t ${topo} --list
    ...    events --format json
        ${output} =    Run Old Cluster Command    ${args}
        Should Not Be Equal As Integers    ${output.rc}    0
        # the error box wraps long lines
        ${message} =    Evaluate    " ".join($output.stderr.split())
        Should Contain    ${message}    no spec.runState
        Should Contain    ${message}    ${required}
    END

    ${namespace} =    Process.Run Process
    ...    kubectl --kubeconfig %{CLAB_C9S_OLD_KUBECONFIG} get namespace ${lab-namespace} --ignore-not-found -o name
    ...    shell=True
    Should Be Empty    ${namespace.stdout}

Offline commands skip the check
    ${validate} =    Run Old Cluster Command    validate -t ${topo}
    Should Be Equal As Integers    ${validate.rc}    0

    ${emit} =    Run Old Cluster Command    deploy -t ${topo} --emit-crs
    Should Be Equal As Integers    ${emit.rc}    0
    Should Contain    ${emit.stdout}    kind: Topology


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'c9s'    This suite targets the c9s runtime.
    ${old} =    Get Environment Variable    CLAB_C9S_OLD_KUBECONFIG    ${EMPTY}
    Skip If    '${old}' == ''    CLAB_C9S_OLD_KUBECONFIG is not set

Run Old Cluster Command
    [Arguments]    ${args}
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} ${args}
    ...    shell=True
    ...    timeout=60s
    ...    env:KUBECONFIG=%{CLAB_C9S_OLD_KUBECONFIG}
    ...    env:CLAB_KUBECONFIG=%{CLAB_C9S_OLD_KUBECONFIG}
    Log    stdout:${\n}${output.stdout}    console=${True}
    Log    stderr:${\n}${output.stderr}    console=${True}
    RETURN    ${output}
