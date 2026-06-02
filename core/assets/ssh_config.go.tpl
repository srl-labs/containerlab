# Containerlab SSH Config for the {{ .TopologyName }} lab

{{- range .Nodes }}
{{- $node := . }}
Host{{- range .Names }} {{ . }}{{- end }}
	{{-  if ne $node.Username ""}}
	User {{ $node.Username }}
	{{- end }}
	StrictHostKeyChecking=no
	UserKnownHostsFile=/dev/null
	{{- if ne $node.SSHConfig.PubkeyAuthentication "" }}
	PubkeyAuthentication={{ $node.SSHConfig.PubkeyAuthentication.String }}
	{{- end }}
{{ end }}