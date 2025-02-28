---
{{- range $kind, $nodes := .Nodes -}}
  {{- $kindProps := index $.Kinds $kind -}}
  {{- range $node := $nodes }}
{{ $node.ShortName }}:
    username: {{ $kindProps.Username }}
    password: {{ $kindProps.Password }}
    platform: {{ $kindProps.Platform }}
    hostname: {{ $node.MgmtIPv4Address }}
  {{- end }}
{{- end }}