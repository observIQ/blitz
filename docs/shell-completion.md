# Shell Completion

Blitz supports shell autocompletion for bash, zsh, fish, and PowerShell. This allows you to use tab completion for commands, flags, and arguments, making the CLI easier to use.

## Linux Package Installation

When you install Blitz using the Linux packages (`.deb` or `.rpm`), **bash completion is automatically installed and enabled** out of the box. No additional configuration is required.

The completion script is installed at `/etc/bash_completion.d/blitz` and will be automatically loaded in new shell sessions.

To use completion in your current session after installation:

```bash
source /etc/bash_completion.d/blitz
```

## Manual Installation

If you installed Blitz manually (downloaded binary, built from source, or using macOS), you can install shell completion using the `completion` command.

### Bash

#### Linux

To install bash completion system-wide (requires root):

```bash
sudo blitz completion bash > /etc/bash_completion.d/blitz
```

To install for your user only:

```bash
mkdir -p ~/.local/share/bash-completion/completions
blitz completion bash > ~/.local/share/bash-completion/completions/blitz
```

#### macOS

If you're using Homebrew's bash-completion:

```bash
blitz completion bash > $(brew --prefix)/etc/bash_completion.d/blitz
```

For other installations:

```bash
mkdir -p ~/.local/share/bash-completion/completions
blitz completion bash > ~/.local/share/bash-completion/completions/blitz
```

#### Loading Completion

After installation, reload your shell configuration:

```bash
source ~/.bashrc
# or
source ~/.bash_profile
```

You can also test completion in the current session:

```bash
source <(blitz completion bash)
```

### Zsh

To install zsh completion:

```bash
blitz completion zsh > "${fpath[1]}/_blitz"
```

Or install to a custom location:

```bash
mkdir -p ~/.zsh/completions
blitz completion zsh > ~/.zsh/completions/_blitz
```

Add the following to your `~/.zshrc` if using a custom location:

```bash
fpath=(~/.zsh/completions $fpath)
autoload -U compinit && compinit
```

To test completion in the current session:

```bash
source <(blitz completion zsh)
```

### Fish

To install fish completion:

```bash
mkdir -p ~/.config/fish/completions
blitz completion fish > ~/.config/fish/completions/blitz.fish
```

To test completion in the current session:

```bash
blitz completion fish | source
```

### PowerShell

To install PowerShell completion for the current session:

```powershell
blitz completion powershell | Out-String | Invoke-Expression
```

To install for all sessions, save the completion script:

```powershell
blitz completion powershell > blitz.ps1
```

Then add the following to your PowerShell profile (usually `$PROFILE`):

```powershell
. .\blitz.ps1
```

## Verifying Installation

After installing completion, you can verify it's working by:

1. Opening a new terminal session
2. Typing `blitz ` (with a space) and pressing `Tab`
3. You should see available commands and flags

Example:

```bash
$ blitz <TAB>
completion  help       version    --config   --generator-type  --output-type  ...
```

## Troubleshooting

### Completion Not Working

1. **Ensure completion script is installed**: Check that the completion file exists in the expected location
2. **Reload your shell**: Close and reopen your terminal, or run `source ~/.bashrc` (or equivalent for your shell)
3. **Check shell compatibility**: Ensure you're using a supported shell (bash, zsh, fish, or PowerShell)
4. **Verify file permissions**: The completion script should be readable

### Linux Package Installation

If completion isn't working after installing via Linux packages:

1. Verify the completion file exists:
   ```bash
   ls -l /etc/bash_completion.d/blitz
   ```

2. Ensure bash-completion is installed:
   ```bash
   # Debian/Ubuntu
   sudo apt-get install bash-completion
   
   # RHEL/CentOS/Fedora
   sudo yum install bash-completion
   # or
   sudo dnf install bash-completion
   ```

3. Source the completion file manually:
   ```bash
   source /etc/bash_completion.d/blitz
   ```

### macOS Issues

If using Homebrew's bash-completion on macOS:

1. Ensure bash-completion is installed:
   ```bash
   brew install bash-completion
   ```

2. Add to your `~/.bash_profile` or `~/.bashrc`:
   ```bash
   [[ -r "$(brew --prefix)/etc/profile.d/bash_completion.sh" ]] && . "$(brew --prefix)/etc/profile.d/bash_completion.sh"
   ```

## Generating Completion Scripts

You can generate completion scripts for any supported shell using the `completion` command:

```bash
# Generate bash completion
blitz completion bash

# Generate zsh completion
blitz completion zsh

# Generate fish completion
blitz completion fish

# Generate PowerShell completion
blitz completion powershell
```

The output can be redirected to a file or piped directly to your shell's configuration.

