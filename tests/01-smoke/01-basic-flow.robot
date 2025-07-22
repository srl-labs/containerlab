*** Settings ***
Library             OperatingSystem
Library             String
Library             Process
Resource            ../common.robot

Suite Teardown      Run    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/01-linux-nodes.clab.yml --cleanup


*** Variables ***
${lab-file}                 01-linux-nodes.clab.yml
${lab-name}                 2-linux-nodes
${runtime}                  docker
# runtime command to execute tasks in a container
# defaults to docker exec. Will be rewritten to containerd `ctr` if needed in "Define runtime exec" test
${runtime-cli-exec-cmd}     sudo docker exec
${n2-ipv4}                  172.20.20.100/24
${n2-ipv6}                  3fff:172:20:20::100/64
${table-delimit}            │
${l1-uname-exec-output}     SEPARATOR=\n
...                         Executed command node=clab-2-linux-nodes-l1 command="uname -n"
...                         ${SPACE}${SPACE}stdout=
...                         ${SPACE}${SPACE}│ l1
${l2-uname-exec-output}     SEPARATOR=\n
...                         Executed command node=clab-2-linux-nodes-l2 command="uname -n"
...                         ${SPACE}${SPACE}stdout=
...                         ${SPACE}${SPACE}│ l2
${l3-uname-exec-output}     SEPARATOR=\n
...                         Executed command node=clab-2-linux-nodes-l3 command="uname -n"
...                         ${SPACE}${SPACE}stdout=
...                         ${SPACE}${SPACE}│ l3


*** Test Cases ***
Verify number of Hosts entries before deploy
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat /etc/hosts | wc -l
    Log    ${output}
    Set Suite Variable    ${HostsFileLines}    ${output}

Deploy ${lab-name} lab
    Log    ${CURDIR}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # save output to be used in next steps
    Set Suite Variable    ${deploy-output}    ${output}

Ensure exec node option works
    [Documentation]    This tests ensures that the node's exec property that sets commands to be executed upon node deployment works.
    # ensure exec commands work
    Should Contain    ${deploy-output}    this_is_an_exec_test
    Should Contain    ${deploy-output}    ID=alpine

Exec command with no filtering
    [Documentation]    This tests ensures that when `exec` command is called without user provided filters, the command is executed on all nodes of the lab.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --cmd 'uname -n'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    # check if output contains the expected log string
    Should Contain
    ...    ${output}    ${l1-uname-exec-output}
    Should Contain
    ...    ${output}    ${l2-uname-exec-output}
    Should Contain
    ...    ${output}    ${l3-uname-exec-output}

Exec command with filtering
    [Documentation]    This tests ensures that when `exec` command is called with user provided filters, the command is executed ONLY on selected nodes of the lab.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label clab-node-name\=l1 --cmd 'uname -n'
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    Should Contain
    ...    ${output}    ${l1-uname-exec-output}

    Should Not Contain    ${output}
    ...    stdout=${\n}${SPACE}${SPACE}│ l2"""
    Should Not Contain    ${output}
    ...    stdout=${\n}${SPACE}${SPACE}│ l3"""

Exec command with json output and filtering
    [Documentation]    This tests ensures that when `exec` command is called with user provided filters and json output, the command is executed ONLY on selected nodes of the lab and the actual JSON is populated to stdout.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label clab-node-name\=l1 --format json --cmd 'cat /test.json' | jq '.[][0].stdout.containerlab'
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    # check if output contains the json value from the /test.json file
    Should Contain    ${output.stdout}    is cool

Ensure CLAB_INTFS env var is set
    [Documentation]
    ...    This test ensures that the CLAB_INTFS environment variable is set in the container
    ...    and that it contains the correct number of interfaces.
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label clab-node-name\=l1 --cmd 'ash -c "echo $CLAB_INTFS"'
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    # l1 node has 3 interfaces defined in the lab topology
    # log outputs to stderr, and thus we check for 3 interfaces there
    # may be worth to change this to stdout in the future
    # we literally check if the string stdout:\n3 is present in the output, as this is how
    # the result is printed today.
    Should Contain    ${output.stderr}
    ...    stdout=
    ...    │ l3

Ensure default no_proxy env var is set
    [Documentation]
    ...    This test ensures that the NO_PROXY env var is populated by clab automatically
    ...    with the relevant addresses and names
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label clab-node-name\=l1 --cmd 'ash -c "echo $NO_PROXY"'
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0

    Should Contain
    ...    ${output.stderr}
    ...    localhost,127.0.0.1,::1,*.local,172.20.20.0/24,172.20.20.100,172.20.20.99,3fff:172:20:20::/64,3fff:172:20:20::100,3fff:172:20:20::99,l1,l2,l3

Inspect ${lab-name} lab using its name
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --name ${lab-name}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Inspect Lab Using JSON Format
    [Documentation]    Verify inspect command with JSON format output using topology file.
    # Run inspect command with JSON format
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} inspect -t ${CURDIR}/${lab-file} --format json
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Not Be Empty    ${output.stdout}

    # Save JSON to a temp file for jq processing
    Create File    /tmp/clab_output.json    ${output.stdout}

    # Check the number of nodes
    ${count_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_output.json | jq -e '."${lab-name}" | length'
    Log    ${count_check}
    Should Be Equal As Integers    ${count_check[0]}    0
    Should Contain    ${count_check[1]}    3

    # Check node name
    ${name_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_output.json | jq -e '."${lab-name}"[0].name'
    Log    ${name_check}
    Should Be Equal As Integers    ${name_check[0]}    0
    Should Contain    ${name_check[1]}    clab-${lab-name}-l1

    # Check node kind
    ${kind_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_output.json | jq -e '."${lab-name}"[1].kind'
    Log    ${kind_check}
    Should Be Equal As Integers    ${kind_check[0]}    0
    Should Contain    ${kind_check[1]}    linux

    # Skip IPv4 check for Podman
    Skip If    '${runtime}' == 'podman'    Skipping IPv4 check for Podman

    # Check IPv4 address
    ${ip_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_output.json | jq -e '."${lab-name}"[1].ipv4_address'
    Log    ${ip_check}
    Should Be Equal As Integers    ${ip_check[0]}    0
    Should Contain    ${ip_check[1]}    ${n2-ipv4}

Inspect Lab Using Details Flag
    [Documentation]    Verify inspect command with --details flag outputs detailed grouped JSON per lab.
    # Run inspect command with --details flag
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} inspect -t ${CURDIR}/${lab-file} --details
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Not Be Empty    ${output.stdout}

    # Save JSON to a temp file for jq processing
    Create File    /tmp/clab_details_output.json    ${output.stdout}

    # Check the key for our lab name is present and is a non-empty array
    ${count_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_details_output.json | jq -e '."${lab-name}" | length'
    Log    ${count_check}
    Should Be Equal As Integers    ${count_check[0]}    0
    Should Contain    ${count_check[1]}    3

    # Check that containers have the expected fields in details view
    ${field_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_details_output.json | jq -e '."${lab-name}"[0] | has("Names") and has("Image") and has("State") and has("Labels")'
    Log    ${field_check}
    Should Be Equal As Integers    ${field_check[0]}    0
    Should Contain    ${field_check[1]}    true

    # Check that the container name matches expected pattern
    ${name_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_details_output.json | jq -e '."${lab-name}"[0].Names[0]'
    Log    ${name_check}
    Should Be Equal As Integers    ${name_check[0]}    0
    Should Contain    ${name_check[1]}    clab-${lab-name}-l1

    # Check that the State is "running"
    ${state_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_details_output.json | jq -e '."${lab-name}"[0].State'
    Log    ${state_check}
    Should Be Equal As Integers    ${state_check[0]}    0
    Should Contain    ${state_check[1]}    running

    # Check that the Labels map includes clab-node-name
    ${label_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_details_output.json | jq -e '."${lab-name}"[0].Labels["clab-node-name"]'
    Log    ${label_check}
    Should Be Equal As Integers    ${label_check[0]}    0
    Should Contain    ${label_check[1]}    l1

    # Check that IP addresses are present
    ${ipv4_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_details_output.json | jq -e '."${lab-name}"[1].NetworkSettings.IPv4addr'
    Log    ${ipv4_check}
    Should Be Equal As Integers    ${ipv4_check[0]}    0
    Should Contain    ${ipv4_check[1]}    172.20.20.100

Inspect Lab Using CSV Format
    [Documentation]    Verify inspect command with CSV format output using topology file.
    # Run inspect command with CSV format
    ${output} =    Process.Run Process
    ...    ${CLAB_BIN} --runtime ${runtime} inspect -t ${CURDIR}/${lab-file} --format csv
    ...    shell=True
    Log    ${output}
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    Should Not Be Empty    ${output.stdout}

    # Save CSV to aa temp file for processing
    Create File    /tmp/clab_output.csv    ${output.stdout}

    # Check the number of nodes
    ${count_check} =    Run And Return Rc And Output
    ...    cat /tmp/clab_output.csv | wc -l
    Log    ${count_check}
    Should Be Equal As Integers    ${count_check[0]}    0
    Should Contain    ${count_check[1]}    3

    # Check node name
    ${name_check} =    Run And Return Rc And Output
    ...    awk -F',' 'NR==2 { print $4 }' /tmp/clab_output.csv
    Log    ${name_check}
    Should Be Equal As Integers    ${name_check[0]}    0
    Should Contain    ${name_check[1]}    clab-${lab-name}-l1

    # Check node kind
    ${kind_check} =    Run And Return Rc And Output
    ...    awk -F',' 'NR==2 { print $7 }' /tmp/clab_output.csv
    Log    ${kind_check}
    Should Be Equal As Integers    ${kind_check[0]}    0
    Should Contain    ${kind_check[1]}    linux

    # Skip IPv4 check for Podman
    Skip If    '${runtime}' == 'podman'    Skipping IPv4 check for Podman

    # Check IPv4 address
    ${ip_check} =    Run And Return Rc And Output
    ...    awk -F',' 'NR==3 { print $10 }' /tmp/clab_output.csv
    Log    ${ip_check}
    Should Be Equal As Integers    ${ip_check[0]}    0
    Should Contain    ${ip_check[1]}    ${n2-ipv4}

Define runtime exec command
    IF    "${runtime}" == "podman"
        Set Suite Variable    ${runtime-cli-exec-cmd}    sudo podman exec
    END

Verify links in node l1
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l1 ip link show eth1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    # testing user-defined MTU is set
    Should Contain    ${output}    2000

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l1 ip link show eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    # testing default MTU is set
    Should Contain    ${output}    9500

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l1 ip link show eth3
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    Should Contain    ${output}    02:00:00:00:00:00

Verify links in node l2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l2 ip link show some1
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l2 ip link show eth2
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l2 ip link show eth3
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    Should Contain    ${output}    02:00:00:00:00:01

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l2 ip link show eth4
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    Should Contain    ${output}    02:00:00:00:00:04

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l2 ip link show eth5
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    Should Contain    ${output}    02:00:00:00:00:05

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-${lab-name}-l2 ip link show somedummy
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    LOWER_UP

Verify links on host
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show l2eth4
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ip link show l2eth5mgmt
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    state UP

Ensure "inspect all" outputs IP addresses
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect --all
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    # get a 4th line from the bottom of the inspect cmd.
    # this relates to the l2 node ipv4
    ${line} =    String.Get Line    ${output}    -6
    Log    ${line}

    @{data} =    Split String    ${line}    ${table-delimit}
    Log    ${data}

    # verify ipv4 address
    ${ipv4} =    String.Strip String    ${data}[6]
    Should Match Regexp    ${ipv4}    ^[\\d\\.]+$

    # get a 3rd line from the bottom of the inspect cmd.
    # this relates to the l2 node ipv6
    ${line} =    String.Get Line    ${output}    -5
    Log    ${line}

    @{data} =    Split String    ${line}    ${table-delimit}
    Log    ${data}

    # verify ipv6 address
    ${ipv6} =    String.Strip String    ${data}[6]
    Run Keyword    Match IPv6 Address    ${ipv6}

Verify "inspect interfaces" contains the expected output
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} inspect interfaces -t ${CURDIR}/${lab-file}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    # Check for presence of node name, interface, interface type and operational state
    Should Contain    ${output}    clab-${lab-name}-l2
    Should Contain    ${output}    eth2
    Should Contain    ${output}    veth
    Should Contain    ${output}    up

Verify bind mount in l1 node
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-2-linux-nodes-l1 cat 01-test.txt
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Hello, containerlab

Verify port forwarding for node l2
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    curl -m 3 --retry 3 localhost:56180
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    Thank you for using nginx

Verify static ipv4 mgmt addressing for l2
    # excluding podman runtime, since static mgmt addressing stopped working in ubuntu 22.04
    # see https://github.com/srl-labs/containerlab/issues/1291
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${ipv4} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-2-linux-nodes-l2 ip -o -4 a sh eth0 | cut -d ' ' -f7
    Log    ${ipv4}
    Should Be Equal As Strings    ${ipv4}    ${n2-ipv4}

Verify static ipv6 mgmt addressing for l2
    # excluding podman runtime, since static mgmt addressing stopped working in ubuntu 22.04
    # see https://github.com/srl-labs/containerlab/issues/1291
    Skip If    '${runtime}' != 'docker'
    ${rc}    ${ipv6} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-2-linux-nodes-l2 ip -o -6 a sh eth0 | cut -d ' ' -f7 | head -1
    Log    ${ipv6}
    Should Be Equal As Strings    ${ipv6}    ${n2-ipv6}

Verify l1 environment has MYVAR variable set
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${runtime-cli-exec-cmd} clab-2-linux-nodes-l1 sh -c "echo \\$MYVAR"
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    MYVAR is SET

Verify Hosts entries exist
    [Documentation]    Verification that the expected /etc/hosts entries are created. We are also checking for the HEADER and FOOTER values here, which also contain the lab name.
    # log host entries for tshooting

    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat /etc/hosts | grep "${lab-name}"

    Log    ${output}

    # not get number of lines related to the current lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat /etc/hosts | grep "${lab-name}" | wc -l

    Log    ${output}

    Should Be Equal As Integers    ${rc}    0

    IF    '${runtime}' == 'podman'    Should Contain    ${output}    6
    IF    '${runtime}' == 'docker'    Should Contain    ${output}    6

Verify Mem and CPU limits are set
    [Documentation]    Checking if cpu and memory limits set for a node has been reflected in the host config
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    sudo ${runtime} inspect clab-${lab-name}-l1 -f '{{.HostConfig.Memory}} {{.HostConfig.CpuQuota}}'
    Log    ${output}
    # cpu=1.5
    Should Contain    ${output}    150000
    # memory=1G
    Should Contain    ${output}    1000000000

Verify iptables allow rule is set
    [Documentation]    Checking if iptables allow rule is set so that external traffic can reach containerlab management network
    Skip If    '${runtime}' != 'docker'
    ${br} =    Run
    ...    sudo ${runtime} inspect clab-${lab-name}-l1 -f '{{index .Config.Labels "clab-mgmt-net-bridge"}}'
    Log    ${br}
    Set Suite Variable    ${MgmtBr}    ${br}
    ${ipt} =    Run
    ...    sudo iptables -vnL DOCKER-USER
    Log    ${ipt}
    # debian 12 uses `0` for protocol, while previous versions use `all`
    # this matches the rule in the in direction
    Should Contain Any    ${ipt}
    ...    ACCEPT all -- * ${MgmtBr}
    ...    ACCEPT 0 -- * ${MgmtBr}
    ...    ignore_case=True
    ...    collapse_spaces=True

    # this matches the rule in the out direction
    Should Contain Any    ${ipt}
    ...    ACCEPT all -- ${MgmtBr} *
    ...    ACCEPT 0 -- ${MgmtBr} *
    ...    ignore_case=True
    ...    collapse_spaces=True

Verify ip6tables allow rule is set
    [Documentation]    Checking if ip6tables allow rule is set so that external traffic can reach containerlab management network
    Skip If    '${runtime}' != 'docker'

    # Add check for ip6tables availability
    ${rc}    ${output} =    Run And Return Rc And Output    which nft
    Skip If    ${rc} != 0    nft command not found

    ${rc}    ${output} =    Run And Return Rc And Output    sudo nft list tables
    Skip If    'ip6 filter' not in '''${output}'''    ip6 filter chain not found

    ${ipt} =    Run
    ...    sudo nft list chain ip6 filter DOCKER-USER
    Log    ${ipt}
    Should Match Regexp    ${ipt}    oifname.*${MgmtBr}.*accept
    Should Match Regexp    ${ipt}    iifname.*${MgmtBr}.*accept

Verify DNS-Server Config
    [Documentation]    Check if the DNS config did take effect
    Skip If    '${runtime}' != 'docker'
    ${output} =    Run
    ...    sudo ${runtime} inspect clab-${lab-name}-l2 -f '{{ .HostConfig.Dns }}'
    Log    ${output}
    Should Contain    ${output}    8.8.8.8
    Should Contain    ${output}    1.2.3.4

Verify DNS-Search Config
    [Documentation]    Check if the DNS config did take effect
    Skip If    '${runtime}' != 'docker'
    ${output} =    Run
    ...    sudo ${runtime} inspect clab-${lab-name}-l2 -f '{{ .HostConfig.DnsSearch }}'
    Log    ${output}
    Should Contain    ${output}    my.domain

Verify DNS-Options Config
    [Documentation]    Check if the DNS config did take effect
    Skip If    '${runtime}' != 'docker'
    ${output} =    Run
    ...    sudo ${runtime} inspect clab-${lab-name}-l2 -f '{{ .HostConfig.DnsOptions }}'
    Log    ${output}
    Should Contain    ${output}    rotate

Verify Exec rc == 0 on containers match
    [Documentation]    Checking that the return code is != 0 if on the exce call not containers match
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --cmd "echo test"
    Log    ${output}
    Should Contain    ${output}    test
    Should Not Contain    ${output}    Filter did not match any containers
    Should Be Equal As Integers    ${rc}    0

Verify Exec rc != 0 on no containers match
    [Documentation]    Checking that the return code is != 0 if on the exce call not containers match
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} exec -t ${CURDIR}/${lab-file} --label clab-node-name=nonexist --cmd "echo test"
    Log    ${output}
    Should Not Contain    ${output}    test
    Should Contain    ${output}    filter did not match any containers
    Should Not Be Equal As Integers    ${rc}    0

Verify l1 node is healthy
    [Documentation]    Checking if l1 node is healthy after the lab is deployed

    Sleep    10s

    ${output} =    Process.Run Process
    ...    sudo ${runtime} inspect clab-${lab-name}-l1 -f ''{{.State.Health.Status}}''
    ...    shell=True
    Log    ${output.stdout}
    Log    ${output.stderr}
    Should Be Equal As Integers    ${output.rc}    0
    # check if output contains the healthy status
    Should Not Contain    ${output.stdout}    unhealthy
    Should Contain    ${output.stdout}    healthy

Destroy ${lab-name} lab
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${CURDIR}/${lab-file} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Verify Hosts entries are gone
    [Documentation]    Verification that the previously created /etc/hosts entries are properly removed. (Again including HEADER and FOOTER).
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat /etc/hosts | grep "${lab-name}" | wc -l
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Should Contain    ${output}    0

Verify Hosts file has same number of lines
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    cat /etc/hosts | wc -l
    Log    ${output}
    Should Be Equal As Integers    ${HostsFileLines}    ${output}

Verify iptables allow rule are gone
    [Documentation]    Checking if iptables allow rule is removed once the lab is destroyed
    Skip If    '${runtime}' != 'docker'
    ${ipt} =    Run
    ...    sudo iptables -vnL DOCKER-USER
    Log    ${ipt}
    Should Not Contain    ${ipt}    ${MgmtBr}

Verify ip6tables allow rule are gone
    [Documentation]    Checking if ip6tables allow rule is removed once the lab is destroyed
    Skip If    '${runtime}' != 'docker'

    # Add check for ip6tables availability
    ${rc}    ${output} =    Run And Return Rc And Output    which nft
    Skip If    ${rc} != 0    nft command not found

    ${rc}    ${output} =    Run And Return Rc And Output    sudo nft list tables
    Skip If    'ip6 filter' not in '''${output}'''    ip6 filter chain not found

    ${ipt} =    Run
    ...    sudo nft list chain ip6 filter DOCKER-USER
    Log    ${ipt}
    Should Not Contain    ${ipt}    ${MgmtBr}

Verify containerlab version
    [Documentation]    Ensures that 'containerlab version' subcommand runs successfully
    ...    and prints basic version fields.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} version
    Log    ${output}
    # The command should exit with 0
    Should Be Equal As Integers    ${rc}    0

    # Check for presence of version/commit/date lines (case-sensitive or partial match)
    Should Contain    ${output}    version:
    Should Contain    ${output}    commit:
    Should Contain    ${output}    date:

Verify containerlab version check
    [Documentation]    Ensures that 'containerlab version check' either says you're on latest version
    ...    or that a new version is available. Also verifies the command succeeds.
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} version check
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

    # On success, containerlab either prints:
    #    "You are on the latest version" OR
    #    "A newer containerlab version (...) is available!"
    # So we allow for either outcome
    Should Contain Any
    ...    ${output}
    ...    You are on the latest version
    ...    A newer containerlab version


*** Keywords ***
Match IPv6 Address
    [Arguments]
    ...    ${address}=${None}
    ${ipv6} =    String.Strip String    ${address}
    Should Match Regexp    ${ipv6}    ^[\\d:abcdef]+$
