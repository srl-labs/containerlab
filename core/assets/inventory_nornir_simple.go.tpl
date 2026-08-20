---
{{- range $kind, $nodes := .Nodes -}}
  {{- $kindProps := index $.Kinds $kind -}}
  {{- range $node := $nodes }}
{{ $node.ShortName }}:
    username: {{if $node.EmitUsernameOnHost}}{{ $node.Credentials.Username }}{{else}}{{ $kindProps.Username }}{{end}}
    password: {{if $node.EmitPasswordOnHost}}{{ $node.Credentials.Password }}{{else}}{{ $kindProps.Password }}{{end}}
    platform: {{ $kindProps.Platform }}
    hostname: {{ $node.MgmtIPv4Address }}
    {{- if $node.NornirGroups }}
    groups:
    {{- range $group := $node.NornirGroups }}
      - {{ $group }}
    {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
