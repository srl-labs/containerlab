*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Setup         Run Keyword    Setup
Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-name}         external-ca
${topo}             ${CURDIR}/09-external-ca.clab.yml
${ca-key-file}      ${CURDIR}/rootCAKey.pem
${ca-cert-file}     ${CURDIR}/rootCACert.pem
${ca-keylength}     2048
${runtime}          docker

# Node based certs files
${l1-key}           ${CURDIR}/clab-${lab-name}/.tls/l1/l1.key
${l1-cert}          ${CURDIR}/clab-${lab-name}/.tls/l1/l1.pem


*** Test Cases ***
Generate CA Key
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl genrsa -out ${ca-key-file} ${ca-keylength}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Generate Certificate
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl req -x509 -sha256 -new -nodes -key ${ca-key-file} -days 3650 -out ${ca-cert-file} -subj "/L=Internet/O=srl-labs/OU=Containerlab/CN=containerlab.dev"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topo}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}

Review Root Certificate
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${ca-cert-file} -text
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Issuer: L = Internet, O = srl-labs, OU = Containerlab, CN = containerlab.dev
    Should Contain    ${output}    Subject: L = Internet, O = srl-labs, OU = Containerlab, CN = containerlab.dev
    Should Contain    ${output}    Public-Key: (${ca-keylength} bit)

Node l1 cert and key files should exist
    File Should Exist    ${l1-cert}
    File Should Exist    ${l1-key}

Review Node l1 Certificate
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${l1-cert} -text
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    CN = l1.external-ca.io
    Should Contain    ${output}    Issuer: L = Internet, O = srl-labs, OU = Containerlab, CN = containerlab.dev
    Should Contain    ${output}    Public-Key: (2048 bit)

Verify node cert with CA Cert
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl verify -CAfile ${ca-cert-file} ${l1-cert}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0


*** Keywords ***
Setup
    Run    rm -f ${ca-key-file} ${ca-cert-file}

Teardown
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup
    Run    rm -f ${ca-key-file} ${ca-cert-file}
