*** Settings ***
Library     OperatingSystem
Library     String
Resource    common.robot


*** Keywords ***
Deploy SR-SIM Lab
    [Arguments]    ${topology}    ${runtime}
    Log    ${topology}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} deploy -t ${topology}
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0
    Check SR-SIM License    ${topology}    ${runtime}

Destroy SR-SIM Lab
    [Arguments]    ${topology}    ${runtime}
    ${rc}    ${output} =    Run And Return Rc And Output
    ...    ${CLAB_BIN} --runtime ${runtime} destroy -t ${topology} --cleanup
    Log    ${output}
    Should Be Equal As Integers    ${rc}    0

Check SR-SIM License
    [Documentation]    After an SR-SIM boots, read its container boot log.
    ...    The license is valid when the boot log reports
    ...    ``Loading TiMOS License ... OK``.
    ...    Expiration is printed only when the log contains it.
    [Arguments]    ${topology}    ${runtime}
    Set Log Level    NONE
    ${status} =    Run Keyword And Return Status
    ...    Wait Until Keyword Succeeds    45 seconds    5 seconds
    ...    Boot Log Contains License Result    ${topology}    ${runtime}
    Set Log Level    INFO
    IF    not ${status}
        Log To Console    License expiration: not found
        Log    License expiration: not found
        Log To Console    License renewal required: yes
        Log    License renewal required: yes
        Fail    SR-SIM boot log has no license result
    END
    Report SR-SIM License From Boot Log

Boot Log Contains License Result
    [Arguments]    ${topology}    ${runtime}
    ${excerpt} =    SR-SIM License Log Excerpt    ${topology}    ${runtime}
    # Wait until the load result is flushed, not only the "Loading" line.
    Should Match Regexp
    ...    ${excerpt}
    ...    Loading TiMOS License for '[^']+' from location:\\s*'[^']+'\\s+\\.\\.\\.\\s+\\S+
    Set Test Variable    ${license_excerpt}    ${excerpt}

SR-SIM License Log Excerpt
    [Arguments]    ${topology}    ${runtime}
    ${topo text} =    OperatingSystem.Get File    ${topology}
    ${labs} =    String.Get Regexp Matches    ${topo text}    (?m)^name:\\s*["']?([^"'\\s#]+)    1
    Should Not Be Empty    ${labs}    Topology ${topology} has no lab name
    ${lab} =    Set Variable    ${labs}[0]
    ${topo dir} =    Evaluate    os.path.dirname($topology)    modules=os
    ${names} =    Run
    ...    ${runtime} ps -aq --filter label=containerlab=${lab} --filter label=clab-node-kind=nokia_srsim --format '{{.Names}}' 2>/dev/null || true
    @{names} =    String.Split To Lines    ${names}
    ${excerpt} =    Set Variable    ${EMPTY}
    FOR    ${name}    IN    @{names}
        ${part} =    Run
        ...    ${runtime} logs '${name}' 2>&1 | grep -a -E -A 3 'Loading TiMOS License|The license installed on|License has expired|expired license' || true
        ${excerpt} =    Catenate    SEPARATOR=\n    ${excerpt}    ${part}
    END
    ${boot} =    Run
    ...    grep -a -R -h -E -A 3 --include=bootlog.txt 'Loading TiMOS License|The license installed on|License has expired|expired license' '${topo dir}/clab-${lab}' 2>/dev/null || true
    ${excerpt} =    Catenate    SEPARATOR=\n    ${excerpt}    ${boot}
    ${excerpt} =    String.Replace String    ${excerpt}    \r    ${EMPTY}
    RETURN    ${excerpt}

Report SR-SIM License From Boot Log
    Set Log Level    NONE
    ${valid} =    Run Keyword And Return Status
    ...    Should Match Regexp
    ...    ${license_excerpt}
    ...    Loading TiMOS License for '[^']+' from location:\\s*'[^']+'\\s+\\.\\.\\.\\s+OK
    ${expiration} =    Set Variable    not found
    ${dates} =    String.Get Regexp Matches
    ...    ${license_excerpt}
    ...    The license installed on \\S+ expires (.+)
    ...    1
    ${expired} =    Run Keyword And Return Status
    ...    Should Match Regexp    ${license_excerpt}    (?i)License has expired|expired license
    ${renewal} =    Set Variable    no
    IF    len($dates) > 0
        ${expiration} =    String.Strip String    ${dates}[0]
        ${renewal} =    Set Variable    yes
    END
    IF    not ${valid} or ${expired}
        ${renewal} =    Set Variable    yes
    END
    Set Log Level    INFO
    Log To Console    License expiration: ${expiration}
    Log    License expiration: ${expiration}
    Log To Console    License renewal required: ${renewal}
    Log    License renewal required: ${renewal}
    IF    '${renewal}' == 'yes'
        Fail    License expiration: ${expiration}. License renewal required: yes
    END
