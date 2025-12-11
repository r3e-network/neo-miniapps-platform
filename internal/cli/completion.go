// Package cli provides shell completion support
package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// BashCompletion generates bash completion script
const BashCompletion = `#!/bin/bash
# Bash completion for service-layer CLI

_service_layer_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main commands
    local commands="gateway oracle vrf neovault secrets neofeeds gasbank neoflow neocompute accounts ccip datalink datastreams dta cre version help completion"

    # Subcommands for each service
    local gateway_cmds="start status stop"
    local oracle_cmds="start status stop"
    local vrf_cmds="start status stop request"
    local neovault_cmds="start status stop deposit withdraw"

    # Global flags
    local global_flags="--help --version --config --log-level --log-format"

    case "${prev}" in
        gateway)
            COMPREPLY=( $(compgen -W "${gateway_cmds} ${global_flags}" -- ${cur}) )
            return 0
            ;;
        oracle)
            COMPREPLY=( $(compgen -W "${oracle_cmds} ${global_flags}" -- ${cur}) )
            return 0
            ;;
        vrf)
            COMPREPLY=( $(compgen -W "${vrf_cmds} ${global_flags}" -- ${cur}) )
            return 0
            ;;
        neovault)
            COMPREPLY=( $(compgen -W "${neovault_cmds} ${global_flags}" -- ${cur}) )
            return 0
            ;;
        --config)
            COMPREPLY=( $(compgen -f -- ${cur}) )
            return 0
            ;;
        --log-level)
            COMPREPLY=( $(compgen -W "debug info warn error" -- ${cur}) )
            return 0
            ;;
        --log-format)
            COMPREPLY=( $(compgen -W "json text" -- ${cur}) )
            return 0
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- ${cur}) )
            return 0
            ;;
        *)
            ;;
    esac

    # Complete main commands
    COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) )
    return 0
}

complete -F _service_layer_completion service-layer
complete -F _service_layer_completion marble
`

// ZshCompletion generates zsh completion script
const ZshCompletion = `#compdef service-layer marble

_service_layer() {
    local -a commands
    commands=(
        'gateway:Start API gateway service'
        'oracle:Start oracle service'
        'vrf:Start VRF service'
        'neovault:Start neovault service'
        'secrets:Start secrets service'
        'neofeeds:Start data feeds service'
        'gasbank:Start gas bank service'
        'neoflow:Start neoflow service'
        'neocompute:Start neocompute service'
        'accounts:Start accounts service'
        'ccip:Start CCIP service'
        'datalink:Start data link service'
        'datastreams:Start data streams service'
        'dta:Start DTA service'
        'cre:Start CRE service'
        'version:Show version information'
        'help:Show help information'
        'completion:Generate shell completion script'
    )

    local -a gateway_cmds
    gateway_cmds=(
        'start:Start the gateway'
        'status:Show gateway status'
        'stop:Stop the gateway'
    )

    local -a vrf_cmds
    vrf_cmds=(
        'start:Start the VRF service'
        'status:Show VRF status'
        'stop:Stop the VRF service'
        'request:Request random number'
    )

    local -a neovault_cmds
    neovault_cmds=(
        'start:Start the neovault service'
        'status:Show neovault status'
        'stop:Stop the neovault service'
        'deposit:Deposit funds'
        'withdraw:Withdraw funds'
    )

    local -a global_flags
    global_flags=(
        '--help[Show help information]'
        '--version[Show version information]'
        '--config[Configuration file path]:file:_files'
        '--log-level[Log level]:level:(debug info warn error)'
        '--log-format[Log format]:format:(json text)'
    )

    _arguments -C \
        '1: :->command' \
        '*:: :->args' \
        $global_flags

    case $state in
        command)
            _describe 'command' commands
            ;;
        args)
            case $words[1] in
                gateway)
                    _describe 'gateway command' gateway_cmds
                    ;;
                vrf)
                    _describe 'vrf command' vrf_cmds
                    ;;
                neovault)
                    _describe 'neovault command' neovault_cmds
                    ;;
                completion)
                    _values 'shell' bash zsh fish
                    ;;
            esac
            ;;
    esac
}

_service_layer "$@"
`

// FishCompletion generates fish completion script
const FishCompletion = `# Fish completion for service-layer CLI

# Main commands
complete -c service-layer -f -n "__fish_use_subcommand" -a "gateway" -d "Start API gateway service"
complete -c service-layer -f -n "__fish_use_subcommand" -a "oracle" -d "Start oracle service"
complete -c service-layer -f -n "__fish_use_subcommand" -a "vrf" -d "Start VRF service"
complete -c service-layer -f -n "__fish_use_subcommand" -a "neovault" -d "Start neovault service"
complete -c service-layer -f -n "__fish_use_subcommand" -a "secrets" -d "Start secrets service"
complete -c service-layer -f -n "__fish_use_subcommand" -a "version" -d "Show version information"
complete -c service-layer -f -n "__fish_use_subcommand" -a "help" -d "Show help information"
complete -c service-layer -f -n "__fish_use_subcommand" -a "completion" -d "Generate shell completion"

# Gateway subcommands
complete -c service-layer -f -n "__fish_seen_subcommand_from gateway" -a "start" -d "Start the gateway"
complete -c service-layer -f -n "__fish_seen_subcommand_from gateway" -a "status" -d "Show gateway status"
complete -c service-layer -f -n "__fish_seen_subcommand_from gateway" -a "stop" -d "Stop the gateway"

# VRF subcommands
complete -c service-layer -f -n "__fish_seen_subcommand_from vrf" -a "start" -d "Start VRF service"
complete -c service-layer -f -n "__fish_seen_subcommand_from vrf" -a "status" -d "Show VRF status"
complete -c service-layer -f -n "__fish_seen_subcommand_from vrf" -a "stop" -d "Stop VRF service"
complete -c service-layer -f -n "__fish_seen_subcommand_from vrf" -a "request" -d "Request random number"

# NeoVault subcommands
complete -c service-layer -f -n "__fish_seen_subcommand_from neovault" -a "start" -d "Start neovault service"
complete -c service-layer -f -n "__fish_seen_subcommand_from neovault" -a "status" -d "Show neovault status"
complete -c service-layer -f -n "__fish_seen_subcommand_from neovault" -a "stop" -d "Stop neovault service"
complete -c service-layer -f -n "__fish_seen_subcommand_from neovault" -a "deposit" -d "Deposit funds"
complete -c service-layer -f -n "__fish_seen_subcommand_from neovault" -a "withdraw" -d "Withdraw funds"

# Completion subcommands
complete -c service-layer -f -n "__fish_seen_subcommand_from completion" -a "bash" -d "Generate bash completion"
complete -c service-layer -f -n "__fish_seen_subcommand_from completion" -a "zsh" -d "Generate zsh completion"
complete -c service-layer -f -n "__fish_seen_subcommand_from completion" -a "fish" -d "Generate fish completion"

# Global flags
complete -c service-layer -l help -d "Show help information"
complete -c service-layer -l version -d "Show version information"
complete -c service-layer -l config -r -d "Configuration file path"
complete -c service-layer -l log-level -x -a "debug info warn error" -d "Log level"
complete -c service-layer -l log-format -x -a "json text" -d "Log format"
`

// GenerateCompletion generates shell completion script
func GenerateCompletion(shell string) error {
	var script string

	switch shell {
	case "bash":
		script = BashCompletion
	case "zsh":
		script = ZshCompletion
	case "fish":
		script = FishCompletion
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
	}

	// Print to stdout
	fmt.Print(script)

	return nil
}

// InstallCompletion installs the completion script to the appropriate location
func InstallCompletion(shell string) error {
	var script string
	var installPath string

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	switch shell {
	case "bash":
		script = BashCompletion
		installPath = filepath.Join(homeDir, ".bash_completion.d", "service-layer")
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
			return fmt.Errorf("failed to create completion directory: %w", err)
		}
	case "zsh":
		script = ZshCompletion
		installPath = filepath.Join(homeDir, ".zsh", "completion", "_service-layer")
		if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
			return fmt.Errorf("failed to create completion directory: %w", err)
		}
	case "fish":
		script = FishCompletion
		installPath = filepath.Join(homeDir, ".config", "fish", "completions", "service-layer.fish")
		if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
			return fmt.Errorf("failed to create completion directory: %w", err)
		}
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	// Write the script
	if err := os.WriteFile(installPath, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to write completion script: %w", err)
	}

	fmt.Printf("Completion script installed to: %s\n", installPath)
	fmt.Println("\nTo enable completion, add the following to your shell config:")

	switch shell {
	case "bash":
		fmt.Println("  source ~/.bash_completion.d/service-layer")
	case "zsh":
		fmt.Println("  fpath=(~/.zsh/completion $fpath)")
		fmt.Println("  autoload -Uz compinit && compinit")
	case "fish":
		fmt.Println("  # Fish will automatically load completions from ~/.config/fish/completions/")
	}

	return nil
}
