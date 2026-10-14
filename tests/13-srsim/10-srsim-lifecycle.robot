*** Settings ***
Library             OperatingSystem
Resource            ../common.robot
Resource            ../srsim.robot

Suite Teardown      Destroy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}


*** Variables ***
${lab-name}         lifecycle-test-srsim
${lab-file-name}    10-srsim-lifecycle.clab.yml
${runtime}          docker


*** Test Cases ***
Deploy ${lab-name} lab
    Deploy SR-SIM Lab    ${CURDIR}/${lab-file-name}    ${runtime}

Ensure client can ping integrated SR OS node
    Sleep    20s    give time for SR OS nodes to boot
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=client --cmd "ping -c2 -W2 10.0.1.2"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    2 packets transmitted, 2 (packets )?received, 0% packet loss

Ensure client can ping distributed SR OS node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=client --cmd "ping -c2 -W2 10.0.2.2"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    2 packets transmitted, 2 (packets )?received, 0% packet loss

Lifecycle stop integrated SR OS node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} stop -t ${CURDIR}/${lab-file-name} -n int
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify ping to integrated SR OS fails after stop
    Sleep    5s    wait for stop to take effect
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=client --cmd "ping -c2 -W2 10.0.1.2"
    Log    ${output}
    Should Contain    ${output}    100% packet loss

Lifecycle start integrated SR OS node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} start -t ${CURDIR}/${lab-file-name} -n int
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify ping to integrated SR OS recovers after start
    Sleep    30s    give time for SR OS to boot
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=client --cmd "ping -c2 -W2 10.0.1.2"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    2 packets transmitted, 2 (packets )?received, 0% packet loss

Lifecycle stop distributed SR OS node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} stop -t ${CURDIR}/${lab-file-name} -n dist
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify ping to distributed SR OS fails after stop
    Sleep    5s    wait for stop to take effect
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=client --cmd "ping -c2 -W2 10.0.2.2"
    Log    ${output}
    Should Contain    ${output}    100% packet loss

Lifecycle start distributed SR OS node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} start -t ${CURDIR}/${lab-file-name} -n dist
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify ping to distributed SR OS recovers after start
    Sleep    30s    give time for SR OS to boot
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=client --cmd "ping -c2 -W2 10.0.2.2"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Match Regexp    ${output}    2 packets transmitted, 2 (packets )?received, 0% packet loss
