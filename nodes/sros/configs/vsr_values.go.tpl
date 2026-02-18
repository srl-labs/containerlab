{
    "network": {
      "interfaces": [
{{- range $i, $name := .Interfaces}}
{{- if $i }},{{ end }}
        {
          "name": "{{$name}}"
        }
{{- end}}
      ]
    },
    "system": {
      "managementInterface": {
        "name": "eth0",
        "ipAddressV4": "{{.MgmtIPv4}}"
{{- if .MgmtIPv6}},
        "ipAddressV6": "{{.MgmtIPv6}}"
{{- end}},
        "staticRoute": "0.0.0.0/0",
        "nextHop": "{{.NextHop}}"
      },
      "dns": {
        "primary": "{{.DNSPrimary}}",
        "domain": "{{.DNSDomain}}"
      },
      "baseMacAddress": "{{.BaseMacAddress}}",
      "mda": [
        {
          "type": "m20-v"
        }
      ],
      "deploymentModel": "{{.DeploymentModel}}"
    }
