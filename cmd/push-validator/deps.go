package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/process"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/pushchain/push-validator-cli/internal/validator"
	"golang.org/x/term"
)

// Prompter abstracts interactive terminal I/O for testability.
type Prompter interface {
	// ReadLine displays the prompt and reads a line of input.
	ReadLine(prompt string) (string, error)
	// IsInteractive returns whether the terminal supports interactive input.
	IsInteractive() bool
}

// CommandRunner abstracts exec.Command calls for testability.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ValidatorFetcher abstracts the cached validator query functions for testability.
type ValidatorFetcher interface {
	GetMyValidator(ctx context.Context, cfg config.Config) (validator.MyValidatorInfo, error)
	GetAllValidators(ctx context.Context, cfg config.Config) (validator.ValidatorList, error)
	GetRewards(ctx context.Context, cfg config.Config, addr string) (commission, outstanding string, err error)
}

// Deps holds all injectable dependencies for command handlers.
type Deps struct {
	Cfg        config.Config
	Sup        process.Supervisor
	Validator  validator.Service
	Node       node.Client
	RemoteNode node.Client // remote RPC client for sync checks
	Fetcher    ValidatorFetcher
	Printer    ui.Printer
	Runner     CommandRunner
	Prompter   Prompter
	Output     io.Writer
	RPCCheck   func(hostport string, timeout time.Duration) bool
}

// execRunner is the production implementation of CommandRunner.
type execRunner struct{}

func (r *execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	// Set DYLD_LIBRARY_PATH for macOS to find libwasmvm.dylib
	// Check multiple potential locations for the dylib
	dylibPaths := []string{}

	// 1. Same directory as binary
	binDir := filepath.Dir(name)
	if binDir != "" && binDir != "." {
		dylibPaths = append(dylibPaths, binDir)
	}

	// 2. Common cosmovisor locations
	homeDir, err := os.UserHomeDir()
	if err == nil {
		cosmovisorDirs := []string{
			filepath.Join(homeDir, ".pchain/cosmovisor/genesis/bin"),
			filepath.Join(homeDir, ".pchain/cosmovisor/current/bin"),
		}
		dylibPaths = append(dylibPaths, cosmovisorDirs...)
	}

	// Build DYLD_LIBRARY_PATH
	if len(dylibPaths) > 0 {
		env := os.Environ()
		existingPath := os.Getenv("DYLD_LIBRARY_PATH")
		newPath := strings.Join(dylibPaths, ":")
		if existingPath != "" {
			newPath = newPath + ":" + existingPath
		}
		env = append(env, "DYLD_LIBRARY_PATH="+newPath)
		cmd.Env = env
	}

	return cmd.Output()
}

// prodFetcher is the production implementation of ValidatorFetcher.
type prodFetcher struct{}

func (f *prodFetcher) GetMyValidator(ctx context.Context, cfg config.Config) (validator.MyValidatorInfo, error) {
	return validator.GetCachedMyValidator(ctx, cfg)
}

func (f *prodFetcher) GetAllValidators(ctx context.Context, cfg config.Config) (validator.ValidatorList, error) {
	return validator.GetCachedValidatorsList(ctx, cfg)
}

func (f *prodFetcher) GetRewards(ctx context.Context, cfg config.Config, addr string) (commission, outstanding string, err error) {
	return validator.GetCachedRewards(ctx, cfg, addr)
}

// ttyPrompter is the production implementation of Prompter.
// It uses /dev/tty when stdin is not a terminal (e.g., piped input).
type ttyPrompter struct{}

func (p *ttyPrompter) ReadLine(prompt string) (string, error) {
	fmt.Print(prompt)

	var reader *bufio.Reader
	if term.IsTerminal(int(os.Stdin.Fd())) {
		reader = bufio.NewReader(os.Stdin)
	} else {
		tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
		if err != nil {
			return "", fmt.Errorf("no interactive terminal available: %w", err)
		}
		defer tty.Close()
		reader = bufio.NewReader(tty)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (p *ttyPrompter) IsInteractive() bool {
	if flagNonInteractive {
		return false
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}
	// Check if /dev/tty is accessible
	tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err == nil {
		tty.Close()
		return true
	}
	return false
}

// newDeps creates production dependencies from the current flags and config.
func newDeps() *Deps {
	cfg := loadCfg()
	bin := findPchaind()
	rpc := cfg.RPCLocal
	if rpc == "" {
		rpc = "http://127.0.0.1:26657"
	}

	return &Deps{
		Cfg:        cfg,
		Sup:        newSupervisor(cfg.HomeDir),
		Printer:    getPrinter(),
		Runner:     &execRunner{},
		Fetcher:    &prodFetcher{},
		Prompter:   &ttyPrompter{},
		Output:     os.Stdout,
		RPCCheck:   process.IsRPCListening,
		Node:       node.New(rpc),
		RemoteNode: node.New(cfg.RemoteRPCURL()),
		Validator: validator.NewWith(validator.Options{
			BinPath:       bin,
			HomeDir:       cfg.HomeDir,
			ChainID:       cfg.ChainID,
			Keyring:       cfg.KeyringBackend,
			GenesisDomain: cfg.GenesisDomain,
			Denom:         cfg.Denom,
		}),
	}
}
