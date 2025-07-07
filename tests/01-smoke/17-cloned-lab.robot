*** Settings ***
Library             Process
Resource            ../common.robot

Test Teardown      Cleanup


*** Variables ***
${lab1-url}             https://github.com/hellt/clab-test-repo
${lab1-shorturl}        hellt/clab-test-repo
${lab1-url2}            https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml
${lab2-url}             https://github.com/hellt/clab-test-repo/tree/branch1

${lab1-gitlab-url}      https://github.com/hellt/clab-test-repo
${lab1-gitlab-url2}     https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml
${lab2-gitlab-url}      https://github.com/hellt/clab-test-repo/tree/branch1
${http-lab-url}         https://gist.githubusercontent.com/hellt/66a5d8fca7bf526b46adae9008a5e04b/raw/034a542c3fbb17333afd20e6e7d21869fee6aeb5/linux.clab.yml
${single-topo-folder}   tests/01-smoke/single-topo-folder

${s3-url}               s3://clab-integration/srl02-s3.clab.yml

${runtime}              docker


*** Test Cases ***
Test lab1 with Github
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1


Test lab1 with Gitlab
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-gitlab-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1


Test lab2 with Github
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-url2}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1


Test lab2 with Gitlab
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-gitlab-url2}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1


Test lab3 with Github
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab2-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab2-node1


Test lab3 with Gitlab
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab2-gitlab-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab2-node1


Test lab1 with short github url
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-shorturl}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    Should Contain    ${output.stdout}    clab-lab1-node1


Test lab1 downloaded from https url
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${http-lab-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    Should Contain    ${output.stdout}    clab-alpine-l1

Test lab downloaded from s3 url
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${s3-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    Should Contain    ${output.stdout}    clab-srl02-srl01

Test deploy referencing folder as topo
    ${output_pre} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${single-topo-folder}
    ...    shell=True

    Log    ${output_pre.stdout}
    Log    ${output_pre.stderr}

    Should Be Equal As Integers    ${output_pre.rc}    0

    ## double check deletion via runtime ps 
    ${output_post2} =    Process.Run Process
    ...    sudo -E ${runtime} ps
    ...    shell=True

    Should Contain    ${output_post2.stdout}    clab-lab1-node1


    ## destroy with just a reference to a folder
    ${output_post1} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${single-topo-folder}
    ...    shell=True

    ## double check deletion via runtime ps 
    ${output_post2} =    Process.Run Process
    ...    sudo -E ${runtime} ps
    ...    shell=True

    Should Not Contain    ${output_post2.stdout}    clab-lab1-node1


*** Keywords ***
Cleanup
    Process.Run Process    ${CLAB_BIN} --runtime ${runtime} destroy --all --cleanup
    ...    shell=True

    Process.Run Process    sudo -E rm -rf clab-test-repo
    ...    shell=True
