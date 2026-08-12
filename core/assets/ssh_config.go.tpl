# Containerlab SSH Config for the {{ .TopologyName }} lab

{{- range .Nodes }}
{{- $node := . }}
Host{{- range .Names }} {{ . }}{{- end }}
	{{-  if ne $node.Username ""}}
	User {{ $node.Username }}
	{{- end }}
	{{- if ne $node.IdentityFile "" }}
	IdentityFile "{{ $node.IdentityFile }}"
	{{- end }}
	StrictHostKeyChecking=no
	UserKnownHostsFile=/dev/null
	{{- if ne $node.SSHConfig.PubkeyAuthentication "" }}
	PubkeyAuthentication={{ $node.SSHConfig.PubkeyAuthentication.String }}
	{{- end }}
{{ end }}