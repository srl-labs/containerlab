{{/* this is a template for sros public key config for ssh admin user access */}}

{{/* to enable long list of keys from agent where the configured key may not be in the default first three keys */}}
/configure system security user-params attempts count 64

{{ range $index, $key := .SSHPubKeysRSA }}
/configure system security user-params local-user user "admin" public-keys rsa rsa-key {{ sub 32 $index }} key-value {{ $key }}
{{ end }}

{{ range $index, $key := .SSHPubKeysECDSA }}
/configure system security user-params local-user user "admin" public-keys ecdsa ecdsa-key {{ sub 32 $index }} key-value {{ $key }}
{{ end }}