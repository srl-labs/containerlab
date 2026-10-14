#!/usr/bin/env bash
set -euo pipefail

# atuin
# bash <(curl --proto '=https' --tlsv1.2 -sSf https://setup.atuin.sh)
curl -LsSf https://github.com/atuinsh/atuin/releases/download/v18.3.0/atuin-installer.sh | ATUIN_NO_MODIFY_PATH=1 sh

ZSH_CUSTOM="${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}"

# zsh-autosuggestions and autocompletions
git clone --depth 1 https://github.com/zsh-users/zsh-autosuggestions "$ZSH_CUSTOM/plugins/zsh-autosuggestions"
git clone --depth 1 https://github.com/marlonrichert/zsh-autocomplete.git "$ZSH_CUSTOM/plugins/zsh-autocomplete"

# syntax highlighting
git clone --depth 1 https://github.com/z-shell/F-Sy-H.git "$ZSH_CUSTOM/plugins/F-Sy-H"

###
### Shell completions
###
# generate containerlab completions
completion_dir="$ZSH_CUSTOM/plugins/zsh-autocomplete/Completions"
/usr/bin/containerlab completion zsh > "$completion_dir/_containerlab"
# add clab alias to the completions
sed -i 's/compdef _containerlab containerlab/compdef _containerlab containerlab clab/g' "$completion_dir/_containerlab"

# uv manages Python versions and virtual environments in all variants.
uv generate-shell-completion zsh > "$completion_dir/_uv"
uvx --generate-shell-completion zsh > "$completion_dir/_uvx"
