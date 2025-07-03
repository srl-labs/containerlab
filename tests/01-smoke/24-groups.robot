*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/24-basic-groups.clab.yaml --cleanup


*** Variables ***
${lab-file}                 24-basic-groups.clab.yaml
${lab-name}                 2-linux-nodes
${runtime}                  docker
# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test
${runtime-cli-exec-cmd}     sudo docker exec
${table-delimit}            │


*** Test Cases ***
Verify number of Hosts entries before deploy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat /etc/hosts | wc -l
    Log    ${output}
    Set Suite Variable    ${HostsFileLines}    ${output}

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}

Inspect ${lab-name} lab using its name
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --name ${lab-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify bind mount in l1 node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-2-linux-nodes-l1 cat 01-test.txt
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Hello, containerlab

Verify port forwarding for node l2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    curl -m 3 --retry 3 localhost:56180
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Thank you for using nginx


Verify l1 environment has MYVAR variable set
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-2-linux-nodes-l1 sh -c "echo \\$MYVAR"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    MYVAR is SET

Verify Mem and CPU limits are set
    [Documentation]    Checking if cpu and memory limits set for a node has been reflected in the host config
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect clab-${lab-name}-l1 -f '{{.HostConfig.Memory}} {{.HostConfig.CpuQuota}}'
    Log    ${output}
    # cpu=1.5
    Should Contain    ${output}    150000
    # memory=1G
    Should Contain    ${output}    1000000000

Verify DNS-Server Config
    [Documentation]    Check if the DNS config did take effect
    Skip If    '${runtime}' != 'docker'
    ${output} =    Run
    ...    sudo ${runtime} inspect clab-${lab-name}-l2 -f '{{ .HostConfig.Dns }}'
    Log    ${output}
    Should Contain    ${output}    8.8.8.8
    Should Contain    ${output}    1.2.3.4

Verify DNS-Search Config
    [Documentation]    Check if the DNS config did take effect
    Skip If    '${runtime}' != 'docker'
    ${output} =    Run
    ...    sudo ${runtime} inspect clab-${lab-name}-l2 -f '{{ .HostConfig.DnsSearch }}'
    Log    ${output}
    Should Contain    ${output}    my.domain

Verify DNS-Options Config
    [Documentation]    Check if the DNS config did take effect
    Skip If    '${runtime}' != 'docker'
    ${output} =    Run
    ...    sudo ${runtime} inspect clab-${lab-name}-l2 -f '{{ .HostConfig.DnsOptions }}'
    Log    ${output}
    Should Contain    ${output}    rotate

Destroy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0