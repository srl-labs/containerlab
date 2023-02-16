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

var NodeCSRTemplate string = `{
    "CN": "{{.Name}}.{{.Prefix}}.io",
    "key": {
      "algo": "rsa",
      "size": 2048
    },
    "names": [{
      "C": "BE",
      "L": "Antwerp",
      "O": "Nokia",
      "OU": "Container lab"
    }],
    "hosts": [
      "{{.Name}}",
      "{{.LongName}}",
      "{{.Fqdn}}"
      {{range .SANs}},"{{.}}"{{end}}
    ]
}
`
