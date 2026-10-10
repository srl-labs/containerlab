*** Settings ***
Documentation       Tailscale attachment on a non-default management network.
...                 No tailnet is joined: the suite checks which containers are created,
...                 the network they join and what they forward to.
Library             OperatingSystem
Library             Collections
Resource            ../common.robot
Resource            mgmt.resource

Suite Setup         Setup
Suite Teardown      Cleanup


*** Variables ***
${runtime}          docker
${key-topo}         ${CURDIR}/11-tailscale-key.clab.yml
${sso-topo}         ${CURDIR}/11-tailscale-sso.clab.yml


*** Test Cases ***
Deploy auth-key lab with tailscale on the second network
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${key-topo}

Sidecar is created only for the node on the tailscale network
    ${containers} =    Command Should Succeed
    ...    docker ps -a --filter label=containerlab=mgmt11k --format '{{.Names}}'
    Should Contain    ${containers.splitlines()}    clab-mgmt11k-b-ts
    Should Not Contain    ${containers.splitlines()}    clab-mgmt11k-a-ts

Sidecar joins the tailscale network and forwards to its parent
    ${sidecar} =    Node Address    clab-mgmt11k-b-ts    clab-mgmt11k-ts
    Address Should Be In Pool    ${sidecar}    198.18.114.0/24
    ${parent} =    Node Address    clab-mgmt11k-b    clab-mgmt11k-ts
    ${env} =    Command Should Succeed
    ...    docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' clab-mgmt11k-b-ts
    Should Contain    ${env.splitlines()}    TS_DEST_IP=${parent}

Destroy removes the sidecar and networks
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${key-topo} --cleanup
    ${containers} =    Command Should Succeed
    ...    docker ps -a --filter label=containerlab=mgmt11k --format '{{.Names}}'
    Should Be Empty    ${containers}
    Network Should Not Exist    clab-mgmt11k-main
    Network Should Not Exist    clab-mgmt11k-ts

Deploy SSO lab with tailscale on the second network
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} deploy -t ${sso-topo}

SSO proxy joins the tailscale network without sidecars
    ${proxy} =    Node Address    clab-mgmt11s-ts    clab-mgmt11s-ts
    Address Should Be In Pool    ${proxy}    198.18.116.0/24
    ${containers} =    Command Should Succeed
    ...    docker ps -a --filter label=containerlab=mgmt11s --format '{{.Names}}'
    Should Not Contain    ${containers.splitlines()}    clab-mgmt11s-a-ts
    Should Not Contain    ${containers.splitlines()}    clab-mgmt11s-b-ts

SSO proxy only serves nodes on the tailscale network
    ${b} =    Node Address    clab-mgmt11s-b    clab-mgmt11s-ts
    ${serve} =    Command Should Succeed    sudo cat ${CURDIR}/clab-mgmt11s/ts/serve.json
    ${serve} =    Evaluate    json.loads($serve)['TCP']    modules=json
    Should Be Equal    ${serve}[9022][TCPForward]    ${b}:22
    Dictionary Should Not Contain Key    ${serve}    8022

Destroy removes the proxy and networks
    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${sso-topo} --cleanup
    ${containers} =    Command Should Succeed
    ...    docker ps -a --filter label=containerlab=mgmt11s --format '{{.Names}}'
    Should Be Empty    ${containers}
    Network Should Not Exist    clab-mgmt11s-main
    Network Should Not Exist    clab-mgmt11s-ts


*** Keywords ***
Setup
    Skip If    '${runtime}' != 'docker'    Multiple management networks require Docker.

Cleanup
    Run Keyword And Ignore Error
    ...    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${key-topo} --cleanup
    Run Keyword And Ignore Error
    ...    Command Should Succeed    ${CLAB_BIN} --runtime ${runtime} destroy -t ${sso-topo} --cleanup
