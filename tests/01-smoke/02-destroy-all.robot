*** Comments ***
This suite tests:
- the destroy --all operation
- the host mode networking for l3 node
- the ipv4-range can be set for a network


*** Settings ***
Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy --all --cleanup


*** Variables ***
${runtime}      docker
${lab1-file}    01-linux-nodes.clab.yml
${lab1-name}    2-linux-nodes
${lab2-file}    01-linux-single-node.clab.yml
${lab2-name}    single-node


*** Test Cases ***
Deploy first lab
    ${result} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab1-file}
    ...    shell=True
    Log    ${result.stdout}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    Should Exist    ${CURDIR}/clab-2-linux-nodes

    Set Suite Variable    ${orig_dir}    ${CURDIR}

Deploy second lab
    ${result} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab2-file}
    ...    cwd=/tmp    # using a different cwd to check lab resolution via container labels
    ...    shell=True
    Log    ${result.stdout}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    Should Exist    ${CURDIR}/clab-single-node

Inspect ${lab2-name} lab using its name
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --name ${lab2-name}
    Log    \n--> LOG: Inspect output\n${output}    console=True
    Should Be Equal As Integers    ${rc}    0

    ${num_lines} =    Run    bash -c "echo '${output}' | wc -l"
    # lab2 only has 1 nodes and therefore inspect output should contain only 1 node with two lines
    # (+4 lines for the table header and footer)
    Should Be Equal As Integers    ${num_lines}    6

Inspect ${lab2-name} lab using topology file reference
    ${result} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} inspect -t ${orig_dir}/${lab2-file}
    ...    shell=True
    Log    \n--> LOG: Inspect output\n${result.stdout}    console=True
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0

    ${num_lines} =    Run    bash -c "echo '${result.stdout}' | wc -l"
    # lab2 only has 1 nodes and therefore inspect output should contain only 1 node
    Should Be Equal As Integers    ${num_lines}    6

Inspect all
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --all
    Log    \n--> LOG: Inspect all output\n${output}    console=True
    Should Be Equal As Integers    ${rc}    0

    ${num_lines} =    Run    bash -c "echo '${output}' | wc -l"
    # 3 nodes in lab1 and 1 node in lab2 (+ for the header, footer and row delimiters)
    Should Be Equal As Integers    ${num_lines}    15

Verify host mode networking for node l3
    # l3 node is launched with host mode networking
    # since it is the nginx container, by launching it in the host mode
    # we should be able to curl localhost:80
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    curl localhost:80
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Thank you for using nginx

Verify ipv4-range is set correctly
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect -t ${CURDIR}/01-linux-single-node.clab.yml
    Log    ${output}
    Should Contain    ${output}    172.20.30.9

Redeploy second lab
    ${result} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} redeploy -c -t ${CURDIR}/${lab2-file}
    ...    cwd=/tmp    # using a different cwd to check lab resolution via container labels
    ...    shell=True
    Log    ${result.stdout}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    Should Exist    ${CURDIR}/clab-${lab2-name}

Destroy by the lab name
    Skip If    '${runtime}' != 'docker'
    ${result} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -c --name ${lab2-name}
    ...    shell=True
    Log    ${result.stdout}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    Should Not Exist    ${CURDIR}/clab-${lab2-name}

Deploy ${lab2-name} lab again
    Skip If    '${runtime}' != 'docker'
    ${result} =    Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab2-file}
    ...    shell=True
    Log    ${result.stdout}
    Log    ${result.stderr}
    Should Be Equal As Integers    ${result.rc}    0
    Should Exist    ${CURDIR}/clab-${lab2-name}

Destroy all labs
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy --all --cleanup --yes
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check all labs have been removed
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --all
    Log    ${output}
    Should Contain    ${output}    no containers found
    Should Not Exist    /tmp/single-node
    Should Not Exist    ${CURDIR}/clab-2-linux-nodes
