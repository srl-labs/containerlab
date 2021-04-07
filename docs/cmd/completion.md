# shell completions

### Description

The `completion` command generates shell completions for bash/zsh/fish shells.

### Usage

`containerlab completion [arg]`

#### Bash completions

```bash
source <(containerlab completion bash)
```

To load completions for each session, execute once:
```bash
# linux
$ containerlab completion bash > /etc/bash_completion.d/containerlab

# macOS
$ containerlab completion bash > /usr/local/etc/bash_completion.d/containerlab
```

#### ZSH completions
If shell completion is not already enabled in your environment, users will need to enable it. To do so, execute the following once:
```bash
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

To load completions for each session, execute once:
```bash
containerlab completion zsh > "${fpath[1]}/_containerlab"
```
> Note: `$fpath[1]` in this command refers to the first path in `$fpath`. Ensure you use
> the index pointing to the completion folder, find the correct index by inspecting the
> output of `echo $fpath`

Start a new shell for this setup to take effect.

#### Fish completions
```bash
containerlab completion fish | source
```

To load completions for each session, execute once:
```
containerlab completion fish > ~/.config/fish/completions/containerlab.fish
```