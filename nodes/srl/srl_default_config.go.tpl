set / system tls server-profile clab-profile
set / system tls server-profile clab-profile key "{{ .TLSKey }}"
set / system tls server-profile clab-profile certificate "{{ .TLSCert }}"

{{- if .TLSAnchor }}
set / system tls server-profile clab-profile authenticate-client true
set / system tls server-profile clab-profile trust-anchor "{{ .TLSAnchor }}"
{{- else }}
set / system tls server-profile clab-profile authenticate-client false
{{- end }}

set / system gnmi-server admin-state enable network-instance mgmt admin-state enable tls-profile clab-profile
set / system gnmi-server rate-limit 65000
set / system gnmi-server trace-options [ request response common ]
set / system gnmi-server unix-socket admin-state enable

{{- if .EnableGNMIUnixSockServices }}
system gnmi-server unix-socket services [ gnmi gnoi ] admin-state enable
{{- end }}

{{- if .DNSServers }}
set / system dns network-instance mgmt
set / system dns server-list [ {{ range $dnsserver := .DNSServers}}{{$dnsserver}} {{ end }}]
{{- end }}

set / system json-rpc-server admin-state enable network-instance mgmt http admin-state enable
set / system json-rpc-server admin-state enable network-instance mgmt https admin-state enable tls-profile clab-profile

{{ .SNMPConfig }}

set / system lldp admin-state enable
set / system aaa authentication idle-timeout 7200

{{- /* if e.g. node is run with none mgmt networking but a macvlan interface is attached as mgmt0, we need to adjust the mtu */}}
{{- if ne .MgmtMTU 0 }}
set / interface mgmt0 mtu {{ .MgmtMTU }}
{{- end }}

{{- if ne .MgmtIPMTU 0 }}
set / interface mgmt0 subinterface 0 ip-mtu {{ .MgmtIPMTU }}
{{- end }}

{{- /* enabling interfaces referenced as endpoints for a node (both e1-2 and e1-3-1 notations) */}}
{{- range $epName, $ep := .IFaces }}
set / interface ethernet-{{ $ep.Slot }}/{{ $ep.Port }} admin-state enable
  {{- if ne $ep.Mtu 0 }}
set / interface ethernet-{{ $ep.Slot }}/{{ $ep.Port }} mtu {{ $ep.Mtu }}
  {{- end }}

  {{- if ne $ep.BreakoutNo  "" }}
set / interface ethernet-{{ $ep.Slot }}/{{ $ep.Port }} breakout-mode num-channels 4 channel-speed 25G
set / interface ethernet-{{ $ep.Slot }}/{{ $ep.Port }}/{{ $ep.BreakoutNo }} admin-state enable
  {{- end }}

{{ end -}}
{{- if .SSHPubKeys }}
set / system aaa authentication linuxadmin-user ssh-key [ {{ .SSHPubKeys }} ]
set / system aaa authentication admin-user ssh-key [ {{ .SSHPubKeys }} ]
{{- end }}
set / system banner login-banner "{{ .Banner }}"

{{- if .EnableCustomPrompt }}
environment save file /home/admin/srlinux_orig.rc
{{- if ne .CustomPrompt  "" }}
environment prompt "{{ .CustomPrompt }}"
environment save file /home/admin/.srlinuxrc
{{- end }}
{{- end }}
