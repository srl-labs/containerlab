# Cert sign

## Description

The `sign` sub-command under the `tools cert` command creates a private key and a certificate and signs the created certificate with a given Certificate Authority.

## Usage

`containerlab tools cert sign [local-flags]`

## Flags

### Name

To set a name under which the certificate and key files will be saved, use the `--name | -n` flag. A name set to `mynode` will create files `mynode.pem`, `mynode.key` and `mynode.csr`.  
The default value is `cert`.

### Path

A directory path under which the generated files will be placed is set with `--path | -p` flag. Defaults to acurrent working directory.

### CA Cert and CA Key

To indicate which CA should sign the certificate request, the provide a path to the CA certificate and key files.

`--ca-cert` flag sets the path to the CA certificate file.  
`--ca-key` flag sets the path to the CA private key file.

### Common Name

Certificate Common Name (CN) field is set with `--cn` flag. Defaults to `containerlab.dev`.

### Hosts

To add Subject Alternative Names (SAN) use the `--hosts` flag that takes a comma separate list of SAN values. Users can provide both DNS names and IP address, and the values will be placed into the DSN SAN and IP SAN automatically.

### Country

Certificate Country (C) field is set with `--country | -c` flag. Defaults to `Internet`.

### Locality

Certificate Locality (L) field is set with `--locality | -l` flag. Defaults to `Server`.

### Organization

Certificate Organization (O) field is set with `--organization | -o` flag. Defaults to `Containerlab`.

### Organization Unit

Certificate Organization Unit (OU) field is set with `--ou` flag. Defaults to `Containerlab Tools`.

### Key size

To set the key size, use the `--key-size` flag. Defaults to `2048`.

## Examples

```bash
# create a private key and certificate and sign the latter
# with the Hosts list of [node.io, 192.168.0.1]
# saving both files under the default name `cert` in the PWD
# and signed by the CA identified by cert ca.pem and key ca-key.pem
containerlab tools cert sign --ca-cert /tmp/ca.pem \
             --ca-key /tmp/ca.key \
             --hosts node.io,192.168.0.1
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
