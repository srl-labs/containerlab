# border0 login

### Description

The `login` sub-command under the `tools border0` command performs a login to border0.com service and saves the acquired authentication token[^1].

The token is saved as `$PWD/.border0_token` file.

### Usage

`containerlab tools border0 login [local-flags]`

### Flags

#### email
With mandatory `--email | -e` flag user sets an email address used to register with border0.com service

#### password
The `--password | -p` sets the password for a user. If flag is not set, the prompt will appear on the terminal to allow for safe enter of the password.

### Examples

```bash
# Login with password entered from the prompt
containerlab tools border0.com login -e myemail@dot.com
Password:
INFO[0000] Written border0.com token to a file /root/containerlab/.border0_token

# Login with password passed as a flag
containerlab tools border0.com login -e myemail@dot.com -p Pa$$word
Password:
INFO[0000] Written border0.com token to a file /root/containerlab/.border0_token
```

[^1]: Authentication token is used to [publish ports](../../../manual/published-ports.md) of a containerlab nodes.