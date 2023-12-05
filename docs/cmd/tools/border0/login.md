# border0 login

### Description

The `login` sub-command under the `tools border0` command performs a login to border0.com service and saves the acquired authentication token[^1].

The token is saved as `$PWD/.border0_token` file.

### Usage

`containerlab tools border0 login [local-flags]`

### Flags

#### disable-browser
The `--disable-browser | -b` prevents the command from attempting to open the browser in order to complete authentication with Border0. If the flag is set, the command will simply print the URL which you must navigate to, whether in the same device or a different device, in order to complete authentication.

### Examples

```bash
containerlab tools border0 login

Please navigate to the URL below in order to complete the login process:
https://portal.border0.com/login?device_identifier=IjM1OTJkZGVmLTgzNTMtNDU4Yy04NjNkLTk1OTdhYjY0ZjFiOSI.ZW6BRw.Z9XlL0CtL7HkKTDX7GSp28d9mG0

Login successful

INFO[0000] Written border0.com token to a file /root/containerlab/.border0_token
```

[^1]: Authentication token is used to [publish ports](../../../manual/published-ports.md) of a containerlab nodes.