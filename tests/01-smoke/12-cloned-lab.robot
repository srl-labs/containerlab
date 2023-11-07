*** Settings ***
Library             Process
Resource            ../common.robot

Suite Setup         Cleanup
Suite Teardown      Cleanup


*** Variables ***
${lab1-url}             https://github.com/hellt/clab-test-repo
${lab1-shorturl}        hellt/clab-test-repo
${lab1-url2}            https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml
${lab2-url}             https://github.com/hellt/clab-test-repo/tree/branch1

${lab1-gitlab-url}      https://github.com/hellt/clab-test-repo
${lab1-gitlab-url2}     https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml
${lab2-gitlab-url}      https://github.com/hellt/clab-test-repo/tree/branch1
${http-lab-url}         https://gist.githubusercontent.com/hellt/66a5d8fca7bf526b46adae9008a5e04b/raw/034a542c3fbb17333afd20e6e7d21869fee6aeb5/linux.clab.yml
${runtime}              docker


*** Test Cases ***
Test lab1 with Github
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1

    Cleanup

Test lab1 with Gitlab
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-gitlab-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1

    Cleanup

Test lab2 with Github
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-url2}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1

    Cleanup

Test lab2 with Gitlab
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-gitlab-url2}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1

    Cleanup

Test lab3 with Github
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab2-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab2-node1

    Cleanup

Test lab3 with Gitlab
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab2-gitlab-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab2-node1

    Cleanup

Test lab1 with short github url
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-shorturl}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1

    Cleanup

Test lab1 downloaded from https url
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${http-lab-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-alpine-l1

    Cleanup


*** Keywords ***
Cleanup
    Process.Run Process    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy --all --cleanup
    ...    shell=True

    Process.Run Process    sudo -E rm -rf clab-test-repo
    ...    shell=True
