# Containerlab SSH Config for the {{ .TopologyName }} lab

{{- range .Nodes }}
{{- $node := . }}
{{- range .Names }}
Host {{ . }}
	{{-  if ne $node.Username ""}}
	User {{ $node.Username }}
	{{- end }}
	StrictHostKeyChecking=no
	UserKnownHostsFile=/dev/null
	{{- if ne $node.SSHConfig.PubkeyAuthentication "" }}
	PubkeyAuthentication={{ $node.SSHConfig.PubkeyAuthentication.String }}
	{{- end }}
{{ end }}
{{- end }}