*** Comments ***
This test suite verifies that `tools cert` commands are working fine

*** Settings ***
Library           OperatingSystem
Library           String
# ensure we remove any certs from prev runs
Suite Setup       Cleanup
Suite Teardown    Cleanup

*** Variables ***
${root-ca-dir}    /tmp/clab-tests/certs/06-ca
${node-cert-dir}    /tmp/clab-tests/certs/06-node-cert

*** Test Cases ***
Create CA certificate
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab tools cert ca create --path ${root-ca-dir} --name root-ca --expiry 1m
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Certificate attributes: CN=containerlab.srlinux.dev, C=Internet, L=Server, O=Containerlab, OU=Containerlab Tools, Validity period=1m

Create and sign end node certificates
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo containerlab tools cert sign --ca-cert ${root-ca-dir}/root-ca.pem --ca-key ${root-ca-dir}/root-ca-key.pem --hosts node.io,192.168.0.1 --path ${node-cert-dir}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Creating and signing certificate: Hosts=[\\"node.io\\" \\"192.168.0.1\\"], CN=containerlab.srlinux.dev, C=Internet, L=Server, O=Containerlab, OU=Containerlab Tools

*** Keywords ***
Cleanup
    Run    rm -rf ${root-ca-dir}
    Run    rm -rf ${node-cert-dir}
