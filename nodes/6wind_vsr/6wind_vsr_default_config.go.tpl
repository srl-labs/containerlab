/ vrf main ssh-server
{{- if .SSHPubKeys }}
/ system auth
/ system auth user admin
/ system auth user admin role admin
{{- range $idx, $key := .SSHPubKeys }}
/ system auth user admin authorized-key "{{ $key }}"
{{- end }}
{{- end }}
{{- if .License }}
/ system license online serial "{{ .License }}"
{{- end }}
{{- if .Banner }}
cmd banner post-login message "{{ .Banner }}"
{{- end }}
