# CA Create

## Description

The `create` sub-command under the `tools cert ca` command creates a Certificate Authority (CA) certificate and its private key.

## Usage

`containerlab tools cert ca create [local-flags]`

## Flags

### Name

To set a name under which the certificate and key files will be saved, use the `--name | -n` flag. A name set to `myname` will create files `myname.pem`, `mynamey.key` and `myname.csr`.  
The default value is `ca`.

### Path

A directory path under which the generated files will be placed is set with `--path | -p` flag. Defaults to current working directory.

### Expiry

Certificate validity period is set as a duration interval with `--expiry | -e` flag. Defaults to `87600h`, which is 10 years.

### Common Name

Certificate Common Name (CN) field is set with `--cn` flag. Defaults to `containerlab.dev`.

### Country

Certificate Country (C) field is set with `--country | -c` flag. Defaults to `Internet`.

### Locality

Certificate Locality (L) field is set with `--locality | -l` flag. Defaults to `Server`.

### Organization

Certificate Organization (O) field is set with `--organization | -o` flag. Defaults to `Containerlab`.

### Organization Unit

Certificate Organization Unit (OU) field is set with `--ou` flag. Defaults to `Containerlab Tools`.

## Examples

```bash
# create CA cert and key in the current dir.
# uses default values for all certificate attributes
# as a result, ca.pem and ca-cert.pem files will be written to the
# current working directory
containerlab tools cert ca create


# create CA cert and key by the specified path with a filename root-ca
# and a validity period of 1 minute
containerlab tools cert ca create --path /tmp/certs/myca --name root-ca \
             --expiry 1m

openssl x509 -in /tmp/certs/myca/root-ca.pem -text | grep -A 2 Validity
        Validity
            Not Before: Mar 25 15:28:00 2021 GMT
            Not After : Mar 25 15:29:00 2021 GMT
```

Generated certificate can be verified/viewed with openssl tool:

```
openssl x509 -in ca.pem -text
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            3f:a7:77:54:e1:2f:47:d6:ca:56:72:e1:d1:d8:c9:0c:e8:46:fd:65
<SNIP>
```
