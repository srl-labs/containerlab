{{ .SNMPConfig }}
{{ .LoggingConfig }}
{{ .GRPCConfig }}
{{- if and .IsSecureGrpc .Tls13CipherCcm8Pre26_7 }}
/configure system security tls server-cipher-list "clab-all" tls13-cipher 5 name tls-aes128-ccm8-sha256
{{- end }}
{{- if .GrpcPre26_3 }}
/configure system grpc md-cli admin-state enable
{{- end }}
{{ .NetconfConfig }}
{{ .SystemConfig }}
{{- if .GrpcPre26_3 }}
/configure system security aaa local-profiles profile "administrative" grpc rpc-authorization md-cli-session permit
{{- end }}
/configure system security user-params local-user user {{ .AdminUser }} restricted-to-home false
/configure system security user-params local-user user {{ .AdminUser }} password {{ .AdminPassword }}
/configure system security user-params local-user user {{ .AdminUser }} access console true
/configure system security user-params local-user user {{ .AdminUser }} console member ["administrative"]
{{- if .PromptPassword }}
/configure system security user-params local-user password minimum-age 0
/configure system security user-params local-user user {{ .AdminUser }} console new-password-at-login true
{{- end }}
{{- if .Name }}
/configure system name {{ .Name }}
/configure system login-control login-banner pre-login-message message "{{ .Banner }}"
{{- end }}
{{ .SSHConfig }}
{{/* this is part takes care of adding public key config for ssh admin user access */}}
{{ range $index, $key := .SSHPubKeysRSA }}
/configure system security user-params local-user user {{ $.AdminUser }} public-keys rsa rsa-key {{ subtract 32 $index }} key-value {{ $key }}
{{ end }}

{{ range $index, $key := .SSHPubKeysECDSA }}
/configure system security user-params local-user user {{ $.AdminUser }} public-keys ecdsa ecdsa-key {{ subtract 32 $index }} key-value {{ $key }}
{{ end }}

{{ range $index, $key := .SSHPubKeysED25519 }}
/configure system security user-params local-user user {{ $.AdminUser }} public-keys ed25519 ed25519-key {{ subtract 32 $index }} key-value {{ $key }}
{{ end }}
{{/* Component configuration for cards, xioms, xiom-mdas and mdas */}}
{{ .ComponentConfig }}
