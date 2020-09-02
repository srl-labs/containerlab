srl_nokia-acl:acl:
  cpm-filter:
    ipv4-filter:
      entry:
      - sequence-id: 10
        description: Accept incoming ICMP unreachable messages
        match:
          icmp:
            code:
            - 0
            - 1
            - 2
            - 3
            - 4
            - 13
            type: dest-unreachable
          protocol: icmp
        action:
          accept:
      - sequence-id: 20
        description: Accept incoming ICMP time-exceeded messages
        match:
          icmp:
            type: time-exceeded
          protocol: icmp
        action:
          accept:
      - sequence-id: 30
        description: Accept incoming ICMP parameter problem messages
        match:
          icmp:
            type: param-problem
          protocol: icmp
        action:
          accept:
      - sequence-id: 40
        description: Accept incoming ICMP echo messages
        match:
          icmp:
            type: echo
          protocol: icmp
        action:
          accept:
      - sequence-id: 50
        description: Accept incoming ICMP echo-reply messages
        match:
          icmp:
            type: echo-reply
          protocol: icmp
        action:
          accept:
      - sequence-id: 60
        description: Accept incoming SSH when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 22
          protocol: tcp
        action:
          accept:
      - sequence-id: 70
        description: Accept incoming SSH when this router initiates the TCP connection
        match:
          protocol: tcp
          source-port:
            operator: eq
            value: 22
        action:
          accept:
      - sequence-id: 80
        description: Accept incoming Telnet when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 23
          protocol: tcp
        action:
          accept:
      - sequence-id: 90
        description: Accept incoming Telnet when this router initiates the TCP connection
        match:
          protocol: tcp
          source-port:
            operator: eq
            value: 23
        action:
          accept:
      - sequence-id: 100 
        description: Accept incoming TACACS+ when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 49
          protocol: tcp
        action:
          accept:
      - sequence-id: 110
        description: Accept incoming TACACS+ when this router initiates the TCP connection
        match:
          protocol: tcp
          source-port:
            operator: eq
            value: 49
        action:
          accept:
      - sequence-id: 120
        description: Accept incoming DNS response messages
        match:
          protocol: udp
          source-port:
            operator: eq
            value: 53
        action:
          accept:
      - sequence-id: 130
        description: Accept incoming DHCP messages targeted for BOOTP/DHCP client
        match:
          destination-port:
            operator: eq
            value: 68
          protocol: udp
        action:
          accept:
      - sequence-id: 140
        description: Accept incoming TFTP read-request and write-request messages
        match:
          destination-port:
            operator: eq
            value: 69
          protocol: udp
        action:
          accept:
      - sequence-id: 150
        description: Accept incoming HTTP(JSON-RPC) when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 80
          protocol: tcp
        action:
          accept:
      - sequence-id: 160
        description: Accept incoming HTTP(JSON-RPC) when this router initiates the TCP connection
        match:
          protocol: tcp
          source-port:
            operator: eq
            value: 80
        action:
          accept:
      - sequence-id: 170
        description: Accept incoming NTP messages from servers
        match:
          protocol: udp
          source-port:
            operator: eq
            value: 123
        action:
          accept:
      - sequence-id: 180
        description: Accept incoming SNMP GET/GETNEXT messages from servers
        match:
          destination-port:
            operator: eq
            value: 161
          protocol: udp
        action:
          accept:
      - sequence-id: 190
        description: Accept incoming BGP when the other router initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 179
          protocol: tcp
        action:
          accept:
      - sequence-id: 200
        description: Accept incoming BGP when this router initiates the TCP connection
        match:
          protocol: tcp
          source-port:
            operator: eq
            value: 179
        action:
          accept:
      - sequence-id: 210
        description: Accept incoming HTTPS(JSON-RPC) when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 443
          protocol: tcp
        action:
          accept:
      - sequence-id: 220
        description: Accept incoming HTTPS(JSON-RPC) when this router initiates the TCP connection
        match:
          protocol: tcp
          source-port:
            operator: eq
            value: 443
        action:
          accept:
      - sequence-id: 230
        description: Accept incoming single-hop BFD session messages
        match:
          destination-port:
            operator: eq
            value: 3784
          protocol: udp
        action:
          accept:
      - sequence-id: 240
        description: Accept incoming multi-hop BFD session messages
        match:
          destination-port:
            operator: eq
            value: 4784
          protocol: udp
        action:
          accept:
      - sequence-id: 250
        description: Accept incoming uBFD session messages
        match:
          destination-port:
            operator: eq
            value: 6784
          protocol: udp
        action:
          accept:
      - sequence-id: 260
        description: Accept incoming gNMI messages when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 57400
          protocol: tcp
        action:
          accept:
      - sequence-id: 270
        description: Accept incoming UDP traceroute messages
        match:
          destination-port:
            range:
              end: 33464
              start: 33434
          protocol: udp
        action:
          accept:
      - sequence-id: 280
        description: Accept incoming ICMP timestamp messages
        match:
          icmp:
            type: timestamp
          protocol: icmp
        action:
          accept:
      - sequence-id: 290 
        action:
          drop:
            log: true
        description: Drop all else
      statistics-per-entry: true
    ipv6-filter:
      entry:
      - sequence-id: 10
        description: Accept incoming ICMPv6 unreachable messages
        match:
          icmp6:
            code:
            - 0
            - 1
            - 2
            - 3
            - 4
            - 5
            - 6
            type: dest-unreachable
          next-header: icmp6      
        action:
          accept:
      - sequence-id: 20
        description: Accept incoming ICMPv6 packet-too-big messages
        match:
          icmp6:
            type: packet-too-big
          next-header: icmp6
        action:
          accept:
      - sequence-id: 30
        description: Accept incoming ICMPv6 time-exceeded messages
        match:
          icmp6:
            type: time-exceeded
          next-header: icmp6
        action:
          accept:
      - sequence-id: 40
        description: Accept incoming ICMPv6 parameter problem messages
        match:
          icmp6:
            type: param-problem
          next-header: icmp6
        action:
          accept:
      - sequence-id: 50
        description: Accept incoming ICMPv6 echo-request messages
        match:
          icmp6:
            type: echo-request
          next-header: icmp6
        action:
          accept:
      - sequence-id: 60
        description: Accept incoming ICMPv6 echo-reply messages
        match:
          icmp6:
            type: echo-reply
          next-header: icmp6
        action:
          accept:
      - sequence-id: 70
        description: Accept incoming ICMPv6 router-advertisement messages
        match:
          icmp6:
            type: router-advertise
          next-header: icmp6
        action:
          accept:
      - sequence-id: 80
        description: Accept incoming ICMPv6 neighbor-solicitation messages
        match:
          icmp6:
            type: neighbor-solicit
          next-header: icmp6
        action:
          accept:
      - sequence-id: 90
        description: Accept incoming ICMPv6 neighbor-advertisement messages
        match:
          icmp6:
            type: neighbor-advertise
          next-header: icmp6
        action:
          accept:
      - sequence-id: 100
        description: Accept incoming SSH when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 22
          next-header: tcp
        action:
          accept:
      - sequence-id: 110
        description: Accept incoming SSH when this router initiates the TCP connection
        match:
          next-header: tcp
          source-port:
            operator: eq
            value: 22
        action:
          accept:
      - sequence-id: 120
        description: Accept incoming Telnet when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 23
          next-header: tcp
        action:
          accept:
      - sequence-id: 130
        description: Accept incoming Telnet when this router initiates the TCP connection
        match:
          next-header: tcp
          source-port:
            operator: eq
            value: 23
        action:
          accept:
      - sequence-id: 140
        description: Accept incoming TACACS+ when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 49
          next-header: tcp
        action:
          accept:
      - sequence-id: 150
        description: Accept incoming TACACS+ when this router initiates the TCP connection
        match:
          next-header: tcp
          source-port:
            operator: eq
            value: 49
        action:
          accept:
      - sequence-id: 160
        description: Accept incoming DNS response messages
        match:
          next-header: udp
          source-port:
            operator: eq
            value: 53
        action:
          accept:
      - sequence-id: 170
        description: Accept incoming TFTP read-request and write-request messages
        match:
          destination-port:
            operator: eq
            value: 69
          next-header: udp
        action:
          accept:
      - sequence-id: 180
        description: Accept incoming HTTP(JSON-RPC) when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 80
          next-header: tcp
        action:
          accept:
      - sequence-id: 190
        description: Accept incoming HTTP(JSON-RPC) when this router initiates the TCP connection
        match:
          next-header: tcp
          source-port:
            operator: eq
            value: 80
        action:
          accept:
      - sequence-id: 200
        description: Accept incoming NTP messages from servers
        match:
          next-header: udp
          source-port:
            operator: eq
            value: 123
        action:
          accept:
      - sequence-id: 210
        description: Accept incoming SNMP GET/GETNEXT messages from servers
        match:
          destination-port:
            operator: eq
            value: 161
          next-header: udp
        action:
          accept:
      - sequence-id: 220
        description: Accept incoming BGP when the other router initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 179
          next-header: tcp
        action:
          accept:
      - sequence-id: 230
        description: Accept incoming BGP when this router initiates the TCP connection
        match:
          next-header: tcp
          source-port:
            operator: eq
            value: 179
        action:
          accept:
      - sequence-id: 240
        description: Accept incoming HTTPS(JSON-RPC) when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 443
          next-header: tcp
        action:
          accept:
      - sequence-id: 250
        description: Accept incoming HTTPS(JSON-RPC) when this router initiates the TCP connection
        match:
          next-header: tcp
          source-port:
            operator: eq
            value: 443
        action:
          accept:
      - sequence-id: 260
        description: Accept incoming DHCPv6 client messages
        match:
          destination-port:
            operator: eq
            value: 546
          next-header: udp
        action:
          accept:
      - sequence-id: 270
        description: Accept incoming single-hop BFD session messages
        match:
          destination-port:
            operator: eq
            value: 3784
          next-header: udp
        action:
          accept:
      - sequence-id: 280
        description: Accept incoming multi-hop BFD session messages
        match:
          destination-port:
            operator: eq
            value: 4784
          next-header: udp
        action:
          accept:
      - sequence-id: 290
        description: Accept incoming uBFD session messages
        match:
          destination-port:
            operator: eq
            value: 6784
          next-header: udp
        action:
          accept:
      - sequence-id: 300
        description: Accept incoming gNMI messages when the other host initiates the TCP connection
        match:
          destination-port:
            operator: eq
            value: 57400
          next-header: tcp
        action:
          accept:
      - sequence-id: 310
        description: Accept incoming UDP traceroute messages
        match:
          destination-port:
            range:
              end: 33464
              start: 33434
          next-header: udp
        action:
          accept:
      - sequence-id: 320
        description: Accept incoming IPV6 hop-in-hop messages
        match:
          next-header: 0
        action:
          accept:
      - sequence-id: 330
        description: Accept incoming IPV6 fragment header messages
        match:
          next-header: 44
        action:
          accept:
      - sequence-id: 340
        action:
          drop:
            log: true
        description: Drop all else

      statistics-per-entry: true

srl_nokia-interfaces:interface:
- name: mgmt0
  admin-state: enable
  subinterface:
  - admin-state: enable
    index: 0
srl_nokia-network-instance:network-instance:
- name: mgmt
  type: srl_nokia-network-instance:ip-vrf
  admin-state: enable
  description: Management network instance
  interface:
  - name: mgmt0.0  
  protocols:
    srl_nokia-linux:linux:
      export-neighbors: true
      export-routes: true
      import-routes: true
srl_nokia-system:system:
  srl_nokia-aaa:aaa:
    authentication:
      authentication-method:
      - local
    server-group:
    - name: local
  srl_nokia-gnmi-server:gnmi-server:
    admin-state: enable
    network-instance:
    - admin-state: enable
      name: mgmt
      port: 57400
      tls-profile: tls-profile-1
      use-authentication: true
    rate-limit: 60
    session-limit: 20
    timeout: 7200
  srl_nokia-ssh:ssh-server:
    network-instance:
    - admin-state: enable
      name: mgmt
  srl_nokia-tls:tls:
    server-profile:
    - name: "tls-profile-1"
{{ if .TLSCert }}      certificate: |
        {{ .TLSCert }}
{{- end}}
{{ if .TLSKey }}      key: |
        {{ .TLSKey }}
{{- end }}
{{ if .TLSAnchor }}      trust-anchor: |
        {{ .TLSAnchor }}
{{- end }}
      authenticate-client: false
