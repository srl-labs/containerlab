# shell completions

## Description

The `completion` command generates shell completions for bash/zsh/fish shells.

## Usage

`containerlab completion [arg]`

### Bash completions

> Ensure that bash-completion is installed on your system.

To load completions for the current session:

```bash
source <(containerlab completion bash)
```

To load completions for each session:

/// tab | Linux

```bash
containerlab completion bash > /etc/bash_completion.d/containerlab
```

///
/// tab | macOS

```bash
containerlab completion bash > /usr/local/etc/bash_completion.d/containerlab
```

///

To also autocomplete for `clab` command alias, add the following to your `.bashrc` or `.bash_profile`:

```bash
complete -o default -F __start_containerlab clab
```

### ZSH completions

If shell completion is not already enabled in your environment you have to enable it by ensuring zsh completions are loaded. The following can be added to your zshrc:

```bash
autoload -U compinit; compinit
```

To load completions for each session generate the completion script and store it somewhere in your `$fpath`:

```bash
clab completion zsh | \
sed '1,2c\#compdef containerlab clab\ncompdef _containerlab containerlab clab' > \
~/.oh-my-zsh/custom/completions/_containerlab
```

/// admonition | Completion script location
    type: subtle-note
The example above uses the `~/.oh-my-zsh/custom/completions` directory, which might not exist by default.

`echo $fpath` will show the directories zsh reads files from. You can either use one of the available completions directories from this list or add a new directory:

```
mkdir -p ~/.oh-my-zsh/custom/completions
```

and then add this directory to the `fpath` list by adding it in your .zshrc file:

```bash
fpath=(~/.oh-my-zsh/custom/completions $fpath)
```

Now you can use `~/.oh-my-zsh/custom/completions` for your completions as shown above.
///

Start a new shell for this setup to take effect.

### Fish completions

```bash
containerlab completion fish | source
```

To load `clab` and `containerlab` completions for each session, execute once:

```
containerlab completion fish > ~/.config/fish/completions/containerlab.fish
echo "complete --command clab --wraps containerlab" > ~/.config/fish/completions/clab.fish
fish_update_completions
```
