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
