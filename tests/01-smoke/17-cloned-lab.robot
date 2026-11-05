*** Settings ***
Library             Process
Library             OperatingSystem
Library             BuiltIn
Resource            ../common.robot

Test Teardown       Cleanup


*** Variables ***
${lab1-url}                 https://github.com/hellt/clab-test-repo
${lab1-shorturl}            hellt/clab-test-repo
${lab1-url2}                https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml
${lab2-url}                 https://github.com/hellt/clab-test-repo/tree/branch1

${lab1-gitlab-url}          https://github.com/hellt/clab-test-repo
${lab1-gitlab-url2}         https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml
${lab2-gitlab-url}          https://github.com/hellt/clab-test-repo/tree/branch1
${http-lab-url}             https://gist.githubusercontent.com/hellt/66a5d8fca7bf526b46adae9008a5e04b/raw/034a542c3fbb17333afd20e6e7d21869fee6aeb5/linux.clab.yml
${single-topo-folder}       tests/01-smoke/single-topo-folder

# the S3 test runs a local MinIO server as an S3 mock.
# official MinIO images are not published to public registries anymore,
# hence the frozen bitnami legacy images are used, pinned by digest.
${s3-bucket}                clab-mock
${s3-key}                   s3-mock.clab.yml
${s3-node}                  clab-s3mock-l1
${s3-user}                  minioadmin
${s3-password}              minioadmin
${s3-endpoint}              http://localhost:9000
${minio-container}          clab-minio-mock
${minio-image}              bitnamilegacy/minio:2025.7.23@sha256:8935e75fa5d11295c17171e4aa49efe390a1193cd7f12e4d21b92af9ffef09d7
${minio-client-image}       bitnamilegacy/minio-client:2025.7.21@sha256:1a2f12802dc75668aded6de1b7b53d67518ac2a7df823760d9f935235fb3d948

${runtime}                  docker


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

Test lab downloaded from s3 url
    Start MinIO Server

    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t s3://${s3-bucket}/${s3-key}
    ...    shell=True
    ...    env:AWS_ENDPOINT_URL=${s3-endpoint}
    ...    env:AWS_ACCESS_KEY_ID=${s3-user}
    ...    env:AWS_SECRET_ACCESS_KEY=${s3-password}

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    Should Contain    ${output.stdout}    ${s3-node}

    # destroy the lab referencing the same s3 url
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t s3://${s3-bucket}/${s3-key} --cleanup
    ...    shell=True
    ...    env:AWS_ENDPOINT_URL=${s3-endpoint}
    ...    env:AWS_ACCESS_KEY_ID=${s3-user}
    ...    env:AWS_SECRET_ACCESS_KEY=${s3-password}

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    ## double check deletion via runtime ps
    ${output_post} =    Process.Run Process
    ...    sudo -E ${runtime} ps
    ...    shell=True

    Should Not Contain    ${output_post.stdout}    ${s3-node}


*** Keywords ***
Cleanup
    Process.Run Process    ${CLAB_BIN} --runtime ${runtime} destroy --all --cleanup
    ...    shell=True

    Process.Run Process    sudo -E rm -rf clab-test-repo
    ...    shell=True

Start MinIO Server
    # drop a leftover container from a previous run, if any
    Process.Run Process    sudo -E ${runtime} rm -f ${minio-container} 2>/dev/null
    ...    shell=True

    ${output} =    Process.Run Process
    ...    sudo -E ${runtime} run --name ${minio-container} --rm -d -p 9000:9000 -e MINIO_ROOT_USER=${s3-user} -e MINIO_ROOT_PASSWORD=${s3-password} ${minio-image}
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0

    Upload S3 Object

Upload S3 Object
    ${output} =    Process.Run Process
    ...    cat ${CURDIR}/${s3-key} | sudo -E ${runtime} run --rm -i --network host --entrypoint /bin/sh ${minio-client-image} -c 'i=0; until mc alias set mock ${s3-endpoint} ${s3-user} ${s3-password}; do i=$((i+1)); [ $i -gt 30 ] && exit 1; sleep 1; done; mc mb --ignore-existing mock/${s3-bucket}; mc pipe mock/${s3-bucket}/${s3-key}'
    ...    shell=True

    Log    ${output.stdout}
    Log    ${output.stderr}

    Should Be Equal As Integers    ${output.rc}    0
