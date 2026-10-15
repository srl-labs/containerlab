#!/usr/bin/env bash
set -euo pipefail

# atuin
# bash <(curl --proto '=https' --tlsv1.2 -sSf https://setup.atuin.sh)
curl -LsSf https://github.com/atuinsh/atuin/releases/download/v18.3.0/atuin-installer.sh | ATUIN_NO_MODIFY_PATH=1 sh

ZSH_CUSTOM="${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}"

# zsh-autosuggestions
git clone --depth 1 https://github.com/zsh-users/zsh-autosuggestions "$ZSH_CUSTOM/plugins/zsh-autosuggestions"

# syntax highlighting
git clone --depth 1 https://github.com/z-shell/F-Sy-H.git "$ZSH_CUSTOM/plugins/F-Sy-H"

###
### Shell completions
###
completion_dir="$ZSH_CUSTOM/completions"
mkdir -p "$completion_dir"

# generate containerlab completions that work for both the
# `containerlab` command and its `clab` alias
# https://containerlab.dev/cmd/completion/#zsh-completions
/usr/bin/containerlab completion zsh |
  sed '1,2c\#compdef containerlab clab\ncompdef _containerlab containerlab clab' > "$completion_dir/_containerlab"

# uv manages Python versions and virtual environments in all variants.
uv generate-shell-completion zsh > "$completion_dir/_uv"
uvx --generate-shell-completion zsh > "$completion_dir/_uvx"
