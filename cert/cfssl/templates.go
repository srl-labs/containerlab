package cfssl

// CACSRTemplate is the template for the CA CSR.
var CACSRTemplate string = `{
    "CN": "{{.CommonName}}",
    "key": {
       "algo": "rsa",
       "size": 2048
    },
    "names": [{
      "C": "{{.Country}}",
      "L": "{{.Locality}}",
      "O": "{{.Organization}}",
      "OU": "{{.OrganizationUnit}}"
    }],
    "ca": {
       "expiry": "{{.Expiry}}"
    }
}
`

// NodeCSRTemplate is the template for the node CSR.
var NodeCSRTemplate string = `{
    "CN": "{{.CommonName}}",
    "hosts": [
      {{- range $i, $e := .Hosts}}
      {{- if $i}},{{end}}
      "{{.}}"
      {{- end}}
    ],
    "key": {
      "algo": "rsa",
      "size": 2048
    },
    "names": [{
      "C": "{{.Country}}",
      "L": "{{.Locality}}",
      "O": "{{.Organization}}",
      "OU": "{{.OrganizationUnit}}"
    }]
  }
`
