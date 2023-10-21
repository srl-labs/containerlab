*** Settings ***
Library             Process
Resource            ../common.robot

Suite Setup         Cleanup
Suite Teardown      Cleanup


*** Variables ***
${lab1-url}                 https://github.com/hellt/clab-test-repo
${lab1-url2}                https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml
${lab2-url}                 https://github.com/hellt/clab-test-repo/tree/branch1
${runtime}                  docker


*** Test Cases ***
Test lab1
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1

    Cleanup

Test lab2
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab1-url2}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab1-node1

    Cleanup

Test lab3
    ${output} =    Process.Run Process
    ...    sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t ${lab2-url}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    # check that node3 was filtered and not present in the lab output
    Should Contain    ${output.stdout}    clab-lab2-node1


*** Keywords ***
Cleanup
    Process.Run Process    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy --all --cleanup
    ...    shell=True

    Process.Run Process    sudo -E rm -rf clab-test-repo
    ...    shell=True
