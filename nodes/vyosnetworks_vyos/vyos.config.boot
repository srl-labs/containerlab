{{- /*
    Vyos requires that the certificate be a single line without the preceding or trailing
    headers https://docs.vyos.io/en/latest/configuration/pki/index.html#key-usage-cli
*/ -}}

{{- $cert := (.TLSCert | strings.ReplaceAll "-----BEGIN CERTIFICATE-----" "" | strings.ReplaceAll "-----END CERTIFICATE-----" "" | strings.ReplaceAll "\n" "") -}}
{{- $key  := (.TLSKey | strings.ReplaceAll "-----BEGIN PRIVATE KEY-----" "" | strings.ReplaceAll "-----END PRIVATE KEY-----" "" | strings.ReplaceAll "\n" "") -}}
{{- $ca   := (.TLSAnchor | strings.ReplaceAll "-----BEGIN CERTIFICATE-----" "" | strings.ReplaceAll "-----END CERTIFICATE-----" "" | strings.ReplaceAll "\n" "") -}}

interfaces {
    ethernet eth0 {
        description "Management Interface"
    }
    loopback lo {
    }
}
pki {
    certificate self {
    certificate "{{ $cert }}"
        private {
        key "{{ $key }}"
        }
    }
    ca clab {
    certificate "{{ $ca }}"
    }
}
service {
    https {
        api {
            keys {
                id admin {
                    key "admin"
                }
            }
        }
        certificates {
            ca-certificate "clab"
            certificate "self"
        }
        listen-address "0.0.0.0"
    }
    ntp {
        allow-client {
            address "127.0.0.0/8"
            address "169.254.0.0/16"
            address "10.0.0.0/8"
            address "172.16.0.0/12"
            address "192.168.0.0/16"
            address "::1/128"
            address "fe80::/10"
            address "fc00::/7"
        }
        server time1.vyos.net {
        }
        server time2.vyos.net {
        }
        server time3.vyos.net {
        }
        
    }
    ssh {
        listen-address "0.0.0.0"
    }
}
system {
    config-management {
        commit-revisions "100"
    }
    console {
        device ttyS0 {
            speed "115200"
        }
    }
host-name {{ .ShortName }}
    login {
        user admin {
            authentication {
                plaintext-password "admin"
            }
        }
    }
    syslog {
        global {
            facility all {
                level "info"
            }
            facility local7 {
                level "debug"
            }
        }
    }
}

// Warning: Do not remove the following line.
// vyos-config-version: "bgp@5:broadcast-relay@1:cluster@2:config-management@1:conntrack@5:conntrack-sync@2:container@2:dhcp-relay@2:dhcp-server@11:dhcpv6-server@5:dns-dynamic@4:dns-forwarding@4:firewall@16:flow-accounting@1:https@6:ids@1:interfaces@33:ipoe-server@4:ipsec@13:isis@3:l2tp@9:lldp@2:mdns@1:monitoring@1:nat@8:nat66@3:ntp@3:openconnect@3:openvpn@4:ospf@2:pim@1:policy@8:pppoe-server@11:pptp@5:qos@2:quagga@11:reverse-proxy@1:rip@1:rpki@2:salt@1:snmp@3:ssh@2:sstp@6:system@27:vrf@3:vrrp@4:vyos-accel-ppp@2:wanloadbalance@3:webproxy@2"
// Release version: 1.5-stream-2025-Q1