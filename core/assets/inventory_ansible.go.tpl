all:
  vars:
    # The generated inventory is assumed to be used from the clab host.
    # Hence no http proxy should be used. Therefore we make sure the http
    # module does not attempt using any global http proxy.
    ansible_httpapi_use_proxy: false
  children:
{{- $root := . }}
{{- range $kind, $nodes := .Nodes}}
    {{$kind}}:
      {{- with index $root.Kinds $kind }}
      {{- if or .NetworkOS .AnsibleConn .Username .Password }}
      vars:
        {{- if .NetworkOS }}
        ansible_network_os: {{ .NetworkOS }}
        {{- end }}
        {{- if .AnsibleConn }}
        # default connection type for nodes of this kind
        # feel free to override this in your inventory
        ansible_connection: {{ .AnsibleConn }}
        {{- else}}
        # ansible_connection: set ansible_connection variable if required
        {{- end }}
        {{- if .Username }}
        ansible_user: {{.Username}}
        {{- end}}
        {{- if .Password }}
        ansible_password: {{.Password}}
        {{- end }}
        {{- end }}
      {{- end }}
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