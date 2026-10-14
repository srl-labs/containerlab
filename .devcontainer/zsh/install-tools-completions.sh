#!/usr/bin/env bash
set -euo pipefail

completion_dir="${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/completions"
mkdir -p "$completion_dir"

# generate gnmic completions
gnmic completion zsh > "$completion_dir/_gnmic"
# generate gnoic completions
gnoic completion zsh > "$completion_dir/_gnoic"
# generate gh completions
gh completion -s zsh > "$completion_dir/_gh"
