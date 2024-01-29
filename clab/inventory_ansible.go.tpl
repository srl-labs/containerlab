all:
  vars:
    # The generated inventory is assumed to be used from the clab host.
    # Hence no http proxy should be used. Therefore we make sure the http
    # module does not attempt using any global http proxy.
    ansible_httpapi_use_proxy: false
  children:
{{- range $kind, $nodes := .Nodes}}
    {{$kind}}:
      hosts:
{{- range $nodes}}
        {{.LongName}}:
    {{- if not (eq (index .Labels "ansible-no-host-var") "true") }}
          ansible_host: {{.MgmtIPv4Address}}
    {{- end -}}
{{- end}}
{{- end}}
{{- range $name, $nodes := .Groups}}
    {{$name}}:
      hosts:
{{- range $nodes}}
        {{.LongName}}:
    {{- if not (eq (index .Labels "ansible-no-host-var") "true") }}
          ansible_host: {{.MgmtIPv4Address}}
    {{- end -}}
{{- end}}
{{- end}}