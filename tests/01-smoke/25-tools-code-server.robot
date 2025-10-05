*** Comments ***
This test suite verifies the functionality of the Containerlab code-server tool operations:
- Starting a code-server container with default settings
- Checking code-server status in table and JSON formats
- Stopping a code-server container
- Starting a code-server container with a custom port
- Verifying cleanup behaviour when no code-server containers are running

*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup Code Server Containers

*** Variables ***
${runtime}              docker
${code_server_name}     clab-code-server
${code_server_image}    ghcr.io/kaelemc/clab-code-server:main
${custom_port}          10080

*** Test Cases ***
Start Code Server With Default Settings
    [Documentation]    Test starting code-server with default parameters
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server start
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    code-server container ${code_server_name} started successfully
    Should Contain    ${output}    code-server available at: http://0.0.0.0:

Check Code Server Status
    [Documentation]    Verify code-server status is reported in table format
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server status
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${code_server_name}
    Should Contain    ${output}    running
    Should Contain    ${output}    ~/.clab

Check Code Server Status JSON Format
    [Documentation]    Verify code-server status is reported in JSON format
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server status --format json
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    "${code_server_name}"
    Should Contain    ${output}    "running"
    Should Contain    ${output}    "labs_dir": "~/.clab"

Stop Code Server
    [Documentation]    Test stopping the default code-server container
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server stop
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Removing code-server container
    Should Contain    ${output}    name=${code_server_name}
    Should Contain    ${output}    code server container removed

    # Verify container is removed
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${runtime} ps -a | grep ${code_server_name} || true
    Log    ${output}
    Should Not Contain    ${output}    ${code_server_name}

Start Code Server With Custom Port
    [Documentation]    Test starting code-server with a custom host port
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server start --port ${custom_port}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    code-server container ${code_server_name} started successfully
    Should Contain    ${output}    code-server available at: http://0.0.0.0:${custom_port}

Verify Code Server Status With Custom Port
    [Documentation]    Verify code-server status reflects the custom port value
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server status
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${code_server_name}
    Should Contain    ${output}    running
    Should Contain    ${output}    ${custom_port}

Stop Code Server Custom Port
    [Documentation]    Stop the code-server container started with custom port
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server stop
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Removing code-server container
    Should Contain    ${output}    name=${code_server_name}
    Should Contain    ${output}    code server container removed

Verify Empty Code Server List
    [Documentation]    Verify status command reports no code-server containers running
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server status
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    No active code-server containers found

Verify Empty Code Server List JSON Format
    [Documentation]    Verify JSON status is empty when no code-server containers exist
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools code-server status --format json
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal    ${output}    []

*** Keywords ***
Cleanup Code Server Containers
    [Documentation]    Cleanup all code-server containers
    Run Keyword And Ignore Error    Run    ${CLAB_BIN} --runtime ${runtime} tools code-server stop --name ${code_server_name}
    Run Keyword And Ignore Error    Run    ${runtime} rm -f ${code_server_name}
