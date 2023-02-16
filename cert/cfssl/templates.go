package cfssl

var rootCACSRTemplate string = `{
    "CN": "{{.Prefix}} Root CA",
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
    "ca": {
       "expiry": "262800h"
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
