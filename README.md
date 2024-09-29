# My dotfiles

Just clone the stuff and symlink the files to appropriate location and enjoy✨

## List of configs

- nvim
- starship
- zsh
- Brewfile
- tmux

## steps to follow

```
# clone the repo
git clone https://github.com/shivamvijaywargi/dotfiles/ ~/.dotfiles

# go to the directory
cd ~/.dotfiles

# To install stuff with homebrew
brew bundle --file ./Brewfile

# create symlinks
# starship config symlink
ln -s ~/.dotfiles/starship.toml ~/.config/starship.toml

# nvim config symlink
ln -s ~/.dotfiles/nvim ~/.config/nvim

# zsh config symlink
ln -s ~/.dotfiles/zshrc ~/.zshrc

# tmux config symlink
ln -s ~/.dotfiles/.tmux.conf ~/.tmux.conf
git clone https://github.com/tmux-plugins/tpm ~/.tmux/plugins/tpm
tmux source ~/.tmux.conf
```
