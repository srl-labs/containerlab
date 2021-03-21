# Mysocketio login

### Description

The `login` sub-command under the `tools mysocketio` command performs a login to mysocketio service and saves the acquired authentication token[^1].

The token is saved as `$PWD/.mysocketio_token` file.

### Usage

`containerlab tools mysocketio login [local-flags]`

### Flags

#### email
With mandatory `--email | -e` flag user sets an email address used to register with mysocketio service

#### password
The `--password | -p` sets the password for a user. If flag is not set, the prompt will appear on the terminal to allow for safe enter of the password.

### Examples

```bash
# Login with password entered from the prompt
containerlab tools mysocketio login -e myemail@dot.com
Password:
INFO[0000] Written mysocketio token to a file /root/containerlab/.mysocketio_token

# Login with password passed as a flag
containerlab tools mysocketio login -e myemail@dot.com -p Pa$$word
Password:
INFO[0000] Written mysocketio token to a file /root/containerlab/.mysocketio_token
```

[^1]: Authentication token is used to [publish ports](../../../manual/published-ports.md) of a containerlab nodes.