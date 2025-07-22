# Containerlab SSH Config for the {{ .TopologyName }} lab

{{- range  .Nodes }}
Host {{ .Name }}
	{{-  if ne .Username ""}}
	User {{ .Username }}
	{{- end }}
	StrictHostKeyChecking=no
	UserKnownHostsFile=/dev/null
	{{- if ne .SSHConfig.PubkeyAuthentication "" }}
	PubkeyAuthentication={{ .SSHConfig.PubkeyAuthentication.String }}
	{{- end }}
{{ end }}