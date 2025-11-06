# CLAB SR OS CLASSIC CONFIGURATION

exit all
configure
#--------------------------------------------------
echo "System Configuration"
#--------------------------------------------------
    system
        name {{ .Name }}
        dns
            address-pref ipv6-first
        exit
        netconf
            listen
                no shutdown
            exit
        exit
        rollback
            rollback-location "cf3:/rollbacks/config"
        exit
        snmp
            streaming
                no shutdown
            exit
            packet-size 9216
        exit
        time
            sntp
                shutdown
            exit
            zone UTC
        exit
    exit
#--------------------------------------------------
echo "System Security Configuration"
#--------------------------------------------------
    system
        security
            profile "administrative"
                grpc
                    rpc-authorization
                        gnoi-cert-mgmt-rotate permit
                        gnoi-cert-mgmt-install permit
                        gnoi-cert-mgmt-getcert permit
                        gnoi-cert-mgmt-revoke permit
                        gnoi-cert-mgmt-cangenerate permit
                        gnoi-system-setpackage permit
                        gnoi-system-switchcontrolprocessor permit
                        gnoi-system-reboot permit
                        gnoi-system-rebootstatus permit
                        gnoi-system-cancelreboot permit
                    exit
                exit
                netconf
                    base-op-authorization
                        action
                        cancel-commit
                        close-session
                        commit
                        copy-config
                        create-subscription
                        delete-config
                        discard-changes
                        edit-config
                        get
                        get-config
                        get-data
                        get-schema
                        kill-session
                        lock
                        validate
                    exit
                exit
                entry 10
                    match "configure system security"
                    action permit
                exit
                entry 20
                    match "show system security"
                    action permit
                exit
                entry 30
                    match "tools perform security"
                    action permit
                exit
                entry 40
                    match "tools dump security"
                    action permit
                exit
                entry 42
                    match "tools dump system security"
                    action permit
                exit
                entry 50
                    match "admin system security"
                    action permit
                exit
                entry 100
                    match "configure li"
                    action deny
                exit
                entry 110
                    match "show li"
                    action deny
                exit
                entry 111
                    match "clear li"
                    action deny
                exit
                entry 112
                    match "tools dump li"
                    action deny
                exit
            exit
            password
                attempts 64 time 5 lockout 10
            exit
            user "admin"
                password "NokiaSros1!"
                access console netconf grpc
                no restricted-to-home
                public-keys
                    rsa
{{ range $index, $key := .SSHPubKeysRSA }}
                        rsa-key {{ subtract 32 $index }} create
                            key-value {{ $key }}
                        exit
{{ end }}
                    exit
                    ecdsa
{{ range $index, $key := .SSHPubKeysECDSA }}
                        ecdsa-key {{ subtract 32 $index }} create
                            key-value {{ $key }}
                        exit
{{ end }}
                    exit
                exit
                console
                    member "administrative"
                exit
            exit
            snmp
                community "public" r version both
                community "private" rwa version v2c
            exit
            per-peer-queuing
            telnet
            exit
        exit
    exit
#--------------------------------------------------
echo "System Login Control Configuration"
#--------------------------------------------------
    system
        login-control
            pre-login-message "{{ .Banner }}"
            login-banner
        exit
    exit
#--------------------------------------------------
echo "Log Configuration"
#--------------------------------------------------
    log
    exit
{{if .SecureGrpc}}
#--------------------------------------------------
echo "System Security Cpm Hw Filters, PKI, TLS and LDAP Configuration"
#--------------------------------------------------
    system
        security
            tls
                cert-profile "clab-grpc-certs" create
                    entry 1 create
                        cert "node.crt"
                        key "node.key"
                    exit
                    shutdown
                exit
                server-cipher-list "clab-all" create
                    cipher 1 name tls-rsa-with3des-ede-cbc-sha
                    cipher 2 name tls-rsa-with-aes128-cbc-sha
                    cipher 3 name tls-rsa-with-aes128-cbc-sha256
                    cipher 4 name tls-rsa-with-aes256-cbc-sha
                    cipher 5 name tls-rsa-with-aes256-cbc-sha256
                    cipher 6 name tls-rsa-with-aes128-gcm-sha256
                    cipher 7 name tls-rsa-with-aes256-gcm-sha384
                    cipher 8 name tls-ecdhe-rsa-aes128-gcm-sha256
                    cipher 9 name tls-ecdhe-rsa-aes256-gcm-sha384
                    tls13-cipher 1 name tls-aes128-gcm-sha256
                    tls13-cipher 2 name tls-aes256-gcm-sha384
                    tls13-cipher 3 name tls-chacha20-poly1305-sha256
                    tls13-cipher 4 name tls-aes128-ccm-sha256
                    tls13-cipher 5 name tls-aes128-ccm8-sha256
                exit
                server-tls-profile "clab-grpc-tls" create
                    cert-profile "clab-grpc-certs"
                    cipher-list "clab-all"
                    no shutdown
                exit
            exit
        exit
    exit
{{end}}
#--------------------------------------------------
echo "System gRPC Configuration"
#--------------------------------------------------
    system
        grpc
{{if .SecureGrpc}}
            tls-server-profile "clab-grpc-tls"
{{else}}
            allow-unsecure-connection
{{end}}
            gnoi
                cert-mgmt
                    no shutdown
                exit
                file
                    no shutdown
                exit
                system
                    no shutdown
                exit
            exit
            md-cli
                no shutdown
            exit
            rib-api
                no shutdown
            exit
            no shutdown
        exit
    exit
#--------------------------------------------------
echo "System Sync-If-Timing Configuration"
#--------------------------------------------------
    system
        sync-if-timing
            begin
            commit
        exit
    exit
#--------------------------------------------------
echo "Management Router Configuration"
#--------------------------------------------------
    router management
    exit

#--------------------------------------------------
echo "Router (Network Side) Configuration"
#--------------------------------------------------
    router Base
    exit

#--------------------------------------------------
echo "Service Configuration"
#--------------------------------------------------
    service
        customer 1 name "1" create
            description "Default customer"
        exit
    exit

#--------------------------------------------------
echo "Log all events for service vprn, log syslog tls-client-profile Configuration"
#--------------------------------------------------
    log
    exit
#--------------------------------------------------
echo "System Configuration Mode Configuration"
#--------------------------------------------------
    system
        management-interface
            configuration-mode classic
            cli
                cli-engine classic-cli md-cli
            exit
        exit
    exit

exit all

