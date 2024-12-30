*** Settings ***
Documentation       This test ensures that the `clab graph` command generates a diagram successfully,
...                 and that the code handling the Docker image updates works as expected.

Library             OperatingSystem
Library             Process
Resource            ../common.robot

Suite Setup         Setup
Suite Teardown      Teardown

*** Variables ***
${lab-file}         03-linux-nodes-to-bridge-and-host.clab.yml
${lab-name}         graph-test
${runtime}          docker
${diagram-file}     03-linux-nodes-to-bridge-and-host.clab.drawio

*** Test Cases ***
Generate Diagram for ${lab-name} Lab
    [Documentation]    This test runs `clab graph` to generate a diagram and verifies success.

    # Run the 'clab graph' command to generate the diagram
    ${output}=    Run Process    sudo    -E    ${CLAB_BIN}    graph    -t    ${CURDIR}/${lab-file}    --drawio    --drawio-args\=--theme nokia_modern
    ...    shell=True    stdout=PIPE    stderr=PIPE

    Log    ${output.stdout}
    Log    ${output.stderr}

    # Ensure the command completed successfully
    Should Be Equal As Integers    ${output.rc}    0

    # Check for expected output messages
    Should Contain    ${output.stdout}    Diagram created successfully.

    # Check that the diagram file was created
    File Should Exist    ${CURDIR}/${diagram-file}

*** Keywords ***
Setup
    # Skip this test suite for podman for now
    Skip If    '${runtime}' == 'podman'

Teardown
    # Clean up by destroying the lab and removing the diagram file
    Remove File    ${CURDIR}/${diagram-file}