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
${scpservername}                scp_server
${ftpservername}                ftp_server
${httpservername}                http_server
${lic_text}                 this is the fake license
# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test
${runtime-cli-exec-cmd}     sudo docker exec


*** Test Cases ***
Deploy helper container - SCP
    ${rc}    ${sshpubkey} =    Run And Return Rc And Output   sudo cat ${idrsa}.pub
    Run    sudo docker run -d --name=${scpservername} -e USER_NAME=user -e PUBLIC_KEY=\"${sshpubkey}\" --restart unless-stopped lscr.io/linuxserver/openssh-server:latest
    #Run    cat ${idrsa}.pub | sudo docker exec -i ${scpservername} tee /root/.ssh/authorized_keys -
    Run    echo "${lic_text} scp" | sudo docker exec -i ${scpservername} tee /config/lic.txt -
    ${rc}    ${scp_server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${scpservername}
    Log    ${scp_server_ip}
    Set Suite Variable    ${scp_server_ip}    ${scp_server_ip}
    Set Environment Variable     CLAB_SSH_KEY    ${idrsa}
    Set Environment Variable     SCP_SERVER_IP    ${scp_server_ip}

Deploy helper container - FTP
    Run    docker run -d --name ${ftpservername} lhauspie/vsftpd-alpine
    ${rc}    ${ftp_server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${ftpservername}
    Log    ${ftp_server_ip}
    Set Suite Variable    ${ftp_server_ip}    ${ftp_server_ip}
    Set Environment Variable     FTP_SERVER_IP    ${ftp_server_ip}
    Run    sudo docker exec -i ${ftpservername} mkdir -p /home/vsftpd/user/
    Run    echo "${lic_text} ftp" | sudo docker exec -i ${ftpservername} tee /home/vsftpd/user/lic.txt -

Deploy helper container - HTTP
    Run    docker run -d --name ${httpservername} httpd:2.4
    Run    echo "${lic_text} http" | sudo docker exec -i ${httpservername} tee /usr/local/apache2/htdocs/lic.txt -
    ${rc}    ${http_server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${httpservername}
    Log    ${http_server_ip}
    Set Suite Variable    ${http_server_ip}    ${http_server_ip}
    Set Environment Variable     HTTP_SERVER_IP    ${http_server_ip}


Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat ${CURDIR}/${lab-file}| envsubst | tee ${CURDIR}/rendered.clab.yml | sudo -E ${CLAB_BIN} --runtime ${runtime} deploy -t -
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}


Check licenses
    ${rc}    ${output} =   Run And Return Rc And Output     cat /tmp/.clab/${lab-name}-l1-lic.txt
    Should Contain    ${output}    ${lic_text} scp
    ${rc}    ${output} =   Run And Return Rc And Output     cat /tmp/.clab/${lab-name}-l2-lic.txt
    Should Contain    ${output}    ${lic_text} ftp
    ${rc}    ${output} =   Run And Return Rc And Output     cat /tmp/.clab/${lab-name}-l3-lic.txt
    Should Contain    ${output}    ${lic_text} http

*** Keywords ***
Teardown
    Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/01-downloads.clab.yaml --cleanup
    Run    rm ${CURDIR}/id_rsa*
    Run    sudo docker rm -f ${scpservername} ${ftpservername} ${httpservername}
    Run    ssh-keygen -f "~/.ssh/known_hosts" -R ${scp_server_ip}
    Run    sudo rm -rf /tmp/.clab/${lab-name}-*

Setup
    Run    ssh-keygen -t ssh-rsa -f ${idrsa} -N ""