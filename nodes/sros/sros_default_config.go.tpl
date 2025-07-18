{{ .SNMPConfig }}
{{ .LoggingConfig }} 
{{ .GRPCConfig }}
{{ .SNMPConfig }}
{{ .NetconfConfig }}
{{ .SystemConfig }}
{{- if .Name }}
configure system name {{ .Name }}
configure system login-control login-banner pre-login-message message "{{ .Banner }}"
{{- end }}
{{ .SSHConfig }}
{{/* this is part takes care of adding public key config for ssh admin user access */}}
{{ range $index, $key := .SSHPubKeysRSA }}
configure system security user-params local-user user "admin" public-keys rsa rsa-key {{ subtract 32 $index }} key-value {{ $key }}
{{ end }}

{{ range $index, $key := .SSHPubKeysECDSA }}
configure system security user-params local-user user "admin" public-keys ecdsa ecdsa-key {{ subtract 32 $index }} key-value {{ $key }}
{{ end }}