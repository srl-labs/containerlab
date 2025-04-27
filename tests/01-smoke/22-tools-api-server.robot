*** Comments ***
This test suite verifies the functionality of the Containerlab API Server operations:
- Starting an API server container with default settings
- Starting an API server with custom port
- Checking API server status
- Testing the health endpoint
- Listing API servers in table and JSON format
- Stopping API server container

*** Settings ***
Library             OperatingSystem
Library             String
Resource            ../common.robot

Suite Teardown      Run Keyword    Cleanup API Server Containers

*** Variables ***
${runtime}              docker
${api_server_name}      clab-api-server
${api_server_image}     ghcr.io/srl-labs/clab-api-server/clab-api-server:latest
${default_port}         8080
${custom_port}          8081

*** Test Cases ***
Start API Server With Default Settings
    [Documentation]    Test starting API server with default parameters
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server start
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    API server container ${api_server_name} started successfully
    Should Contain    ${output}    API Server available at: http://localhost:8080

Test API Server Health Endpoint
    [Documentation]    Test the API server health endpoint
    # Give the server a moment to fully start
    Sleep    15s

    ${rc}    ${output}=    Run And Return Rc And Output
    ...    curl -s http://localhost:8080/health -H 'accept: application/json'
    Log    Health check output:
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    "status":"healthy"
    Should Contain    ${output}    "uptime":
    Should Contain    ${output}    "startTime":
    Should Contain    ${output}    "version":

Check API Server Status
    [Documentation]    Test checking API server status in table format
    # Get container logs before status check
    ${rc}    ${logs}=    Run And Return Rc And Output
    ...    ${runtime} logs ${api_server_name}
    Log    Container logs before status check: ${logs}

    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server status
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${api_server_name}

    # Check if container is still running and handle both cases
    ${is_running}=    Evaluate    'running' in '''${output}''' or 'exited' in '''${output}'''
    Should Be True    ${is_running}    Status table should show either running or exited state
    Should Contain    ${output}    localhost
    Should Contain    ${output}    8080

Check API Server Status JSON Format
    [Documentation]    Test checking API server status in JSON format
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server status --format json
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    "${api_server_name}"
    Should Contain    ${output}    "running"
    Should Contain    ${output}    "host": "localhost"
    Should Contain    ${output}    "port": 8080

Stop API Server
    [Documentation]    Test stopping API server container
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server stop
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    API server container ${api_server_name} removed successfully

    # Verify container is removed
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${runtime} ps -a | grep ${api_server_name} || true
    Log    ${output}
    Should Not Contain    ${output}    ${api_server_name}

Start API Server With Custom Port
    [Documentation]    Test starting API server with custom port configuration
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server start --port ${custom_port} --log-level info
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    API server container ${api_server_name} started successfully
    Should Contain    ${output}    API Server available at: http://localhost:8081

Test API Server Health Endpoint Custom Port
    [Documentation]    Test the API server health endpoint on custom port
    # Give the server a moment to fully start
    Sleep    15s

    ${rc}    ${output}=    Run And Return Rc And Output
    ...    curl -s http://localhost:8081/health -H 'accept: application/json'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    "status":"healthy"
    Should Contain    ${output}    "uptime":
    Should Contain    ${output}    "startTime":
    Should Contain    ${output}    "version":

Verify API Server Status With Custom Port
    [Documentation]    Verify API server status shows custom port
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server status
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    ${api_server_name}
    Should Contain    ${output}    running
    Should Contain    ${output}    localhost
    Should Contain    ${output}    8081

Final Stop API Server
    [Documentation]    Stop the API server for final cleanup
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server stop
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    API server container ${api_server_name} removed successfully

Verify Empty API Server List
    [Documentation]    Test that no API servers are listed after stopping
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server status
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    No active API server containers found

Verify Empty API Server List JSON Format
    [Documentation]    Test that empty JSON array is returned when no API servers are running
    ${rc}    ${output}=    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} tools api-server status --format json
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Be Equal    ${output}    []

*** Keywords ***
Cleanup API Server Containers
    [Documentation]    Cleanup all API server containers
    Run Keyword And Ignore Error    Run    ${CLAB_BIN} --runtime ${runtime} tools api-server stop --name ${api_server_name}
    Run Keyword And Ignore Error    Run    ${runtime} rm -f ${api_server_name}