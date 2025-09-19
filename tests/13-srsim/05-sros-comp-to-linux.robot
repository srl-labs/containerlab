*** Settings ***
Library             OperatingSystem
Resource            ../ssh.robot
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup


*** Variables ***
${lab-name}         sr05
${lab-file-name}    05-srsim-comp.clab.yml
${runtime}          docker
${key-name}         clab-test-key


*** Test Cases ***
Set key-path Variable
    ${key-path} =    OperatingSystem.Normalize Path    ~/.ssh/${key-name}
    Set Suite Variable    ${key-path}

Create SSH keypair - RSA
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ssh-keygen -t rsa -N "" -f ${key-path}-rsa

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Ensure sros is reachable over ssh
    Login via SSH with username and password
    ...    address=clab-${lab-name}-sros-a
    ...    username=admin
    ...    password=NokiaSros1!
    ...    try_for=10

Verify links in node l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ip link show eth1"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Ensure l1 can ping sros over 1/1/c23/4 interface
    Sleep    30s    give some time for linecards to come up
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file-name} --label clab-node-name\=l1 --cmd "ping 10.0.0.2 -c2 -w 3"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0% packet loss

Ensure MDA is overriden with explicit slot on sr1-01
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show mda" | sshpass -p "NokiaSros1!" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-sr1-01
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    me12-100gb-qsfp28

Ensure MDA is overriden with implicit slot on sr1-02
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show mda" | sshpass -p "NokiaSros1!" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-sr1-02
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    me12-100gb-qsfp28

Ensure MDA is overriden with env var on component on sr1-03
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show mda" | sshpass -p "NokiaSros1!" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-sr1-03
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    me12-100gb-qsfp28

Ensure MDA is overriden with env var on node on sr1-04
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show mda" | sshpass -p "NokiaSros1!" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-sr1-04
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    me12-100gb-qsfp28

Ensure XIOM is equipped and up on sr1-05
    Sleep    60s    give some time for linecards to come up
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show xiom | match up | match up" | sshpass -p "NokiaSros1!" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-sr1-05-a
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    iom-s-3.0t

Ensure XIOM MDA x/1 is equipped and up on sr1-05
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    echo "show mda | match x1/1 | match up | match up" | sshpass -p "NokiaSros1!" ssh -o "IdentitiesOnly=yes" admin@clab-${lab-name}-sr1-05-a
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ms18-100gb-qsfp28

*** Keywords ***
Cleanup
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file-name} --cleanup
    Run    rm -f ${key-path}*
