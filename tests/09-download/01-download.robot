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
${sshpwservername}              ssh_pw_server
${ftpservername}                ftp_server
${httpservername}                http_server
${httpsservername}               https_server
${lic_text}                 this is the fake license
${ssh_password}             pass
# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test
${runtime-cli-exec-cmd}     sudo docker exec


*** Test Cases ***
Deploy helper container - SCP
    ${rc}    ${sshpubkey} =    Run And Return Rc And Output   sudo cat ${idrsa}.pub
    Run    sudo docker run -d --name=${scpservername} -e USER_NAME=user -e PUBLIC_KEY=\"${sshpubkey}\" --restart unless-stopped lscr.io/linuxserver/openssh-server:latest
    #Run    cat ${idrsa}.pub | sudo docker exec -i ${scpservername} tee /root/.ssh/authorized_keys -
    Run    echo "${lic_text} scp" | sudo docker exec -i ${scpservername} tee /config/scp-license.key -
    Run    echo "${lic_text} sftp" | sudo docker exec -i ${scpservername} tee /config/sftp-license.key -
    Run    echo '{"download": "startup config scp"}' | sudo docker exec -i ${scpservername} tee /config/scp-startup.json -
    Run    echo '{"download": "startup config sftp"}' | sudo docker exec -i ${scpservername} tee /config/sftp-startup.json -
    ${rc}    ${scp_server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${scpservername}
    Log    ${scp_server_ip}
    Set Suite Variable    ${scp_server_ip}    ${scp_server_ip}
    Set Environment Variable     CLAB_SSH_KEY    ${idrsa}
    Set Environment Variable     SCP_SERVER_IP    ${scp_server_ip}

Deploy helper container - SSH password
    Run    sudo docker run -d --name=${sshpwservername} -e USER_NAME=user -e PASSWORD_ACCESS=true -e USER_PASSWORD=${ssh_password} --restart unless-stopped lscr.io/linuxserver/openssh-server:latest
    Run    echo "${lic_text} scp password" | sudo docker exec -i ${sshpwservername} tee /config/scp-password-license.key -
    Run    echo "${lic_text} sftp password" | sudo docker exec -i ${sshpwservername} tee /config/sftp-password-license.key -
    Run    echo '{"download": "startup config scp password"}' | sudo docker exec -i ${sshpwservername} tee /config/scp-password-startup.json -
    Run    echo '{"download": "startup config sftp password"}' | sudo docker exec -i ${sshpwservername} tee /config/sftp-password-startup.json -
    ${rc}    ${ssh_pw_server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${sshpwservername}
    Log    ${ssh_pw_server_ip}
    Set Suite Variable    ${ssh_pw_server_ip}    ${ssh_pw_server_ip}
    Set Environment Variable     SSH_PW_SERVER_IP    ${ssh_pw_server_ip}
    Set Environment Variable     SSH_PASSWORD    ${ssh_password}

Deploy helper container - FTP
    Run    docker run -d --name ${ftpservername} lhauspie/vsftpd-alpine
    ${rc}    ${ftp_server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${ftpservername}
    Log    ${ftp_server_ip}
    Set Suite Variable    ${ftp_server_ip}    ${ftp_server_ip}
    Set Environment Variable     FTP_SERVER_IP    ${ftp_server_ip}
    Run    sudo docker exec -i ${ftpservername} mkdir -p /home/vsftpd/user/
    Run    echo "${lic_text} ftp" | sudo docker exec -i ${ftpservername} tee /home/vsftpd/user/license.key -
    Run    echo '{"download": "startup config ftp"}' | sudo docker exec -i ${ftpservername} tee /home/vsftpd/user/startup.json -

Deploy helper container - HTTP
    Run    docker run -d --name ${httpservername} httpd:2.4
    Run    echo "${lic_text} http" | sudo docker exec -i ${httpservername} tee /usr/local/apache2/htdocs/license.key -
    Run    echo '{"download": "startup config http"}' | sudo docker exec -i ${httpservername} tee /usr/local/apache2/htdocs/startup.json -
    ${rc}    ${http_server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${httpservername}
    Log    ${http_server_ip}
    Set Suite Variable    ${http_server_ip}    ${http_server_ip}
    Set Environment Variable     HTTP_SERVER_IP    ${http_server_ip}

Deploy helper container - HTTPS
    Run    docker run -d --name ${httpsservername} httpd:2.4
    Run    echo "${lic_text} https" | sudo docker exec -i ${httpsservername} tee /usr/local/apache2/htdocs/license.key -
    Run    echo '{"download": "startup config https"}' | sudo docker exec -i ${httpsservername} tee /usr/local/apache2/htdocs/startup.json -
    Run    sudo docker exec ${httpsservername} openssl req -x509 -nodes -days 1 -newkey rsa:2048 -keyout /usr/local/apache2/conf/server.key -out /usr/local/apache2/conf/server.crt -subj "/CN=containerlab-test"
    Run    sudo docker exec ${httpsservername} sed -i "s/#LoadModule ssl_module/LoadModule ssl_module/" /usr/local/apache2/conf/httpd.conf
    Run    sudo docker exec ${httpsservername} sed -i "s/#LoadModule socache_shmcb_module/LoadModule socache_shmcb_module/" /usr/local/apache2/conf/httpd.conf
    Run    sudo docker exec ${httpsservername} sed -i "s/#Include conf\\/extra\\/httpd-ssl.conf/Include conf\\/extra\\/httpd-ssl.conf/" /usr/local/apache2/conf/httpd.conf
    Run    sudo docker exec ${httpsservername} httpd -k restart
    ${rc}    ${https_server_ip} =    Run And Return Rc And Output
    ...    sudo docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${httpsservername}
    Log    ${https_server_ip}
    Set Suite Variable    ${https_server_ip}    ${https_server_ip}
    Set Environment Variable     HTTPS_SERVER_IP    ${https_server_ip}


Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    envsubst < ${CURDIR}/${lab-file} > ${CURDIR}/rendered.clab.yml && sudo -E ${CLAB_BIN} --runtime ${runtime} deploy --skip-post-deploy -t ${CURDIR}/rendered.clab.yml
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}

Check startup configs in artifacts
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l1/config/config.json
    Should Contain    ${output}    startup config scp
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l2/config/config.json
    Should Contain    ${output}    startup config ftp
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l3/config/config.json
    Should Contain    ${output}    startup config http
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l4/config/config.json
    Should Contain    ${output}    startup config sftp
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l5/config/config.json
    Should Contain    ${output}    startup config https
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l6/config/config.json
    Should Contain    ${output}    startup config scp password
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l7/config/config.json
    Should Contain    ${output}    startup config sftp password

Check startup configs in containers
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l1 cat /etc/opt/srlinux/config.json
    Should Contain    ${output}    startup config scp
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l2 cat /etc/opt/srlinux/config.json
    Should Contain    ${output}    startup config ftp
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l3 cat /etc/opt/srlinux/config.json
    Should Contain    ${output}    startup config http
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l4 cat /etc/opt/srlinux/config.json
    Should Contain    ${output}    startup config sftp
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l5 cat /etc/opt/srlinux/config.json
    Should Contain    ${output}    startup config https
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l6 cat /etc/opt/srlinux/config.json
    Should Contain    ${output}    startup config scp password
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l7 cat /etc/opt/srlinux/config.json
    Should Contain    ${output}    startup config sftp password

Check licenses in artifacts
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l1/license.key
    Should Contain    ${output}    ${lic_text} scp
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l2/license.key
    Should Contain    ${output}    ${lic_text} ftp
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l3/license.key
    Should Contain    ${output}    ${lic_text} http
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l4/license.key
    Should Contain    ${output}    ${lic_text} sftp
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l5/license.key
    Should Contain    ${output}    ${lic_text} https
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l6/license.key
    Should Contain    ${output}    ${lic_text} scp password
    ${rc}    ${output} =   Run And Return Rc And Output     cat ${CURDIR}/clab-${lab-name}/l7/license.key
    Should Contain    ${output}    ${lic_text} sftp password

Check licenses in containers
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l1 cat /opt/srlinux/etc/license.key
    Should Contain    ${output}    ${lic_text} scp
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l2 cat /opt/srlinux/etc/license.key
    Should Contain    ${output}    ${lic_text} ftp
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l3 cat /opt/srlinux/etc/license.key
    Should Contain    ${output}    ${lic_text} http
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l4 cat /opt/srlinux/etc/license.key
    Should Contain    ${output}    ${lic_text} sftp
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l5 cat /opt/srlinux/etc/license.key
    Should Contain    ${output}    ${lic_text} https
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l6 cat /opt/srlinux/etc/license.key
    Should Contain    ${output}    ${lic_text} scp password
    ${rc}    ${output} =   Run And Return Rc And Output     ${runtime-cli-exec-cmd} clab-${lab-name}-l7 cat /opt/srlinux/etc/license.key
    Should Contain    ${output}    ${lic_text} sftp password

*** Keywords ***
Teardown
    Run    sudo -E ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/rendered.clab.yml --cleanup
    Run    rm -f ${CURDIR}/id_rsa*
    Run    sudo docker rm -f ${scpservername} ${sshpwservername} ${ftpservername} ${httpservername} ${httpsservername}
    Run    ssh-keygen -f "~/.ssh/known_hosts" -R ${scp_server_ip}
    Run    ssh-keygen -f "~/.ssh/known_hosts" -R ${ssh_pw_server_ip}
    Run    rm -f ${CURDIR}/rendered.clab.yml
    Run    sudo rm -rf ${CURDIR}/clab-${lab-name}
    Run    sudo rm -rf /tmp/.clab/${lab-name}-*

Setup
    Run    ssh-keygen -t ssh-rsa -f ${idrsa} -N ""
