*** Comments ***
This test suite verifies that `tools cert` commands are working fine


*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

# ensure we remove any certs from prev runs
Suite Setup         Cleanup
Suite Teardown      Cleanup


*** Variables ***
${root-ca-dir}      /tmp/clab-tests/certs/06-ca
${node-cert-dir}    /tmp/clab-tests/certs/06-node-cert


*** Test Cases ***
Create CA certificate
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} tools cert ca create --path ${root-ca-dir} --name root-ca --expiry 1m --locality CICD -d
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain
    ...    ${output}
    ...    Certificate attributes: CN=containerlab.dev, C=Internet, L=CICD, O=Containerlab, OU=Containerlab Tools, Validity period=1m
    # check the cert contents with openssl
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${root-ca-dir}/root-ca.pem -text
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain
    ...    ${output}
    ...    Issuer: C = Internet, L = CICD, O = Containerlab, OU = Containerlab Tools, CN = containerlab.dev

Create and sign end node certificates
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} tools cert sign --ca-cert ${root-ca-dir}/root-ca.pem --ca-key ${root-ca-dir}/root-ca.key --hosts node.io,192.168.0.1 --path ${node-cert-dir} -d
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain
    ...    ${output}
    ...    Creating and signing certificate Hosts="[node.io 192.168.0.1]" CN=containerlab.dev C=Internet L=Server O=Containerlab OU="Containerlab Tools"
    # check the cert contents with openssl
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    openssl x509 -in ${node-cert-dir}/cert.pem -text
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    DNS:node.io, IP Address:192.168.0.1


*** Keywords ***
Cleanup
    Run    rm -rf ${root-ca-dir}
    Run    rm -rf ${node-cert-dir}
