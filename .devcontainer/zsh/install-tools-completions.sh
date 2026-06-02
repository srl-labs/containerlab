#!/usr/bin/env bash
gnmic completion zsh > "/home/vscode/.oh-my-zsh/custom/plugins/zsh-autocomplete/Completions/_gnmic"
# generate gnoic completions
gnoic completion zsh > "/home/vscode/.oh-my-zsh/custom/plugins/zsh-autocomplete/Completions/_gnoic"
# generate gh
gh completion -s zsh > "/home/vscode/.oh-my-zsh/custom/plugins/zsh-autocomplete/Completions/_gh"
