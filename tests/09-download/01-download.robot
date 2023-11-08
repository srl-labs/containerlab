*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot
Suite Setup         Setup
Suite Teardown      Teardown


*** Variables ***
${lab-file}                 01-downloads.clab.yaml
${lab-name}                 01-downloads
${idrsa}                    ${CURDIR}/id_rsa
${serverhelpername}         server_helper
${lic_text}                 this is the fake license
# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test
${runtime-cli-exec-cmd}     sudo docker exec


*** Test Cases ***

Deploy helper container
    Run    sudo docker run -d --name ${serverhelpername} dlc
    Run    sudo docker exec ${serverhelpername} mkdir -p /root/.ssh/
    Run    cat ${idrsa}.pub | sudo docker exec -i ${serverhelpername} tee /root/.ssh/authorized_keys -
    Run    echo "${lic_text}" | sudo docker exec -i ${serverhelpername} tee /root/lic.txt -
    ${rc}    ${server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${serverhelpername}
    Log    ${server_ip}
    Set Suite Variable    ${server_ip}    ${server_ip}
    Set Environment Variable     CLAB_SSH_KEY    ${idrsa}
    Set Environment Variable     SERVER_IP    ${server_ip}

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat ${CURDIR}/${lab-file}| envsubst | sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t -
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}


Check license
    ${rc}    ${output} =   Run And Return Rc And Output     cat /tmp/.clab/${lab-name}-l1-lic.txt
    Should Contain    ${output}    ${lic_text}


*** Keywords ***
Teardown
    Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/01-downloads.clab.yaml --cleanup
    Run    rm ${CURDIR}/id_rsa*
    Run    sudo docker stop ${serverhelpername}
    Run    sudo docker rm ${serverhelpername}
    Run    ssh-keygen -f "~/.ssh/known_hosts" -R ${server_ip}

Setup
    Run    ssh-keygen -t ssh-rsa -f ${idrsa} -N ""