# Q pre block. Keep at the top of this file.
[[ -f "${HOME}/Library/Application Support/amazon-q/shell/zshrc.pre.zsh" ]] && builtin source "${HOME}/Library/Application Support/amazon-q/shell/zshrc.pre.zsh"

# Add deno completions to search path
if [[ ":$FPATH:" != *":/Users/shivam/.zsh/completions:"* ]]; then export FPATH="/Users/shivam/.zsh/completions:$FPATH"; fi

eval $(thefuck --alias)
# You can use whatever you want as an alias, like for Mondays:
eval $(thefuck --alias FUCK)

# Path to your oh-my-zsh installation.
export ZSH="$HOME/.oh-my-zsh"

plugins=(git web-search history macos z thefuck)

# Custom Aliases
alias c="code ."
alias n="nvim"
alias finder="open -a finder"

source $(brew --prefix)/share/zsh-autosuggestions/zsh-autosuggestions.zsh
source /opt/homebrew/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh
source $ZSH/oh-my-zsh.sh

# bun completions
[ -s "/Users/shivam/.bun/_bun" ] && source "/Users/shivam/.bun/_bun"

# bun
export BUN_INSTALL="$HOME/.bun"
export PATH="$BUN_INSTALL/bin:$PATH"export GPG_TTY=$(tty)

eval "$(fzf --zsh)"

export PATH=$PATH:$(go env GOPATH)/bin

export GPG_TTY=$(tty)

eval "$(starship init zsh)"

. "/Users/shivam/.deno/env"

# Q post block. Keep at the bottom of this file.
[[ -f "${HOME}/Library/Application Support/amazon-q/shell/zshrc.post.zsh" ]] && builtin source "${HOME}/Library/Application Support/amazon-q/shell/zshrc.post.zsh"
