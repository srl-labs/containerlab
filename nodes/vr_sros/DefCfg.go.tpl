{{/* this is a template for sros public key config for ssh admin user access */}}

{{ range $index, $key := .SSHPubKeysRSA }}
/configure system security user-params local-user user "admin" public-keys rsa rsa-key {{ add $index 1 }} key-value {{ $key }}
{{ end }}

{{ range $index, $key := .SSHPubKeysECDSA }}
/configure system security user-params local-user user "admin" public-keys ecdsa ecdsa-key {{ add $index +1 }} key-value {{ $key }}
{{ end }}
