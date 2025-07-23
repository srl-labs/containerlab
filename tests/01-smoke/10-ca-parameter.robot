*** Settings ***
Library             OperatingSystem
Library             String
Library             DateTime
Resource            ../common.robot

Suite Teardown      Run Keyword    Teardown


*** Variables ***
${lab-name}                 internal-ca
${topo}                     ${CURDIR}/10-${lab-name}.clab.yml
${ca-keysize}               1024
${l1-keysize}               1024
${l1-validity-duration}     25 hours
${l2-keysize}               2048
${ca-validity-duration}     5 hours

# cert files
${ca-cert-key}              ${CURDIR}/clab-${lab-name}/.tls/ca/ca.key
${ca-cert-file}             ${CURDIR}/clab-${lab-name}/.tls/ca/ca.pem
${l1-key}                   ${CURDIR}/clab-${lab-name}/.tls/l1/l1.key
${l1-cert}                  ${CURDIR}/clab-${lab-name}/.tls/l1/l1.pem
${l2-key}                   ${CURDIR}/clab-${lab-name}/.tls/l2/l2.key
${l2-cert}                  ${CURDIR}/clab-${lab-name}/.tls/l2/l2.pem
${l3-key}                   ${CURDIR}/clab-${lab-name}/.tls/l3/l3.key
${l3-cert}                  ${CURDIR}/clab-${lab-name}/.tls/l3/l3.pem


*** Test Cases ***
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
    Should Contain    ${output}    Issuer: C = US, L = , O = containerlab, OU = , CN = ${lab-name} lab CA
    Should Contain    ${output}    Subject: C = US, L = , O = containerlab, OU = , CN = ${lab-name} lab CA
    Should Contain    ${output}    Public-Key: (${ca-keysize} bit)

Node l1 cert and key files should exist
    File Should Exist    ${l1-cert}
    File Should Exist    ${l1-key}

Node l2 cert and key files should exist
    File Should Exist    ${l2-cert}
    File Should Exist    ${l2-key}

Node l3 cert and key files should not exist
    File Should Not Exist    ${l3-cert}
    File Should Not Exist    ${l3-key}

Review Node l1 Certificate
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${l1-cert} -text
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    CN = l1.${lab-name}.io
    Should Contain    ${output}    Issuer: C = US, L = , O = containerlab, OU = , CN = ${lab-name} lab CA
    Should Contain    ${output}    Public-Key: (${l1-keysize} bit)

Review Node l2 Certificate
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${l2-cert} -text
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    CN = l2.${lab-name}.io
    Should Contain    ${output}    Issuer: C = US, L = , O = containerlab, OU = , CN = ${lab-name} lab CA
    Should Contain    ${output}    Public-Key: (${l2-keysize} bit)

Verfiy node cert l1 with CA Cert
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl verify -CAfile ${ca-cert-file} ${l1-cert}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verfiy node cert l2 with CA Cert
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl verify -CAfile ${ca-cert-file} ${l2-cert}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify CA Certificate Validity
    ${rc}    ${certificate_output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${ca-cert-file} -text
    Check Certificat Validity Duration    ${certificate_output}    ${ca-validity-duration}

Verify l1 Certificate Validity
    ${rc}    ${certificate_output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${l1-cert} -text
    Check Certificat Validity Duration    ${certificate_output}    ${l1-validity-duration}

Verify l2 extra SANs
    ${rc}    ${certificate_output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${l2-cert} -text
    Should Contain    ${certificate_output}    DNS:my.text.fqdn
    Should Contain    ${certificate_output}    IP Address:192.168.33.44


*** Keywords ***
Teardown
    Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topo} --cleanup

Get Certificate Date
    [Arguments]    ${certificate_output}    ${type}
    ${date} =    Get Regexp Matches
    ...    ${certificate_output}
    ...    Not ${type}\\W*: (\\w{3}\\W+\\d{1,2} \\d{2}:\\d{2}:\\d{2} \\d{4} \\w{3})
    ...    1
    RETURN    ${date}[0]

Check Certificat Validity Duration
    [Arguments]    ${certificate_output}    ${expected_duration}
    ${not_before} =    Get Certificate Date    ${certificate_output}    Before
    ${not_after} =    Get Certificate Date    ${certificate_output}    After

    ${time_difference} =    Subtract Date From Date
    ...    ${not_after}
    ...    ${not_before}
    ...    date1_format=%b %d %H:%M:%S %Y %Z
    ...    date2_format=%b %d %H:%M:%S %Y %Z

    ${verbose_time_difference} =    Convert Time    ${time_difference}    verbose

    ${expected_verbose_time_difference} =    Convert Time    ${expected_duration}    verbose

    Should Be Equal    ${verbose_time_difference}    ${expected_verbose_time_difference}
