package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pushchain/push-validator-cli/internal/exitcodes"
	syncmon "github.com/pushchain/push-validator-cli/internal/sync"
)

// SyncRunner abstracts the sync monitor for testability.
type SyncRunner interface {
	Run(ctx context.Context, opts syncmon.Options) error
}

// prodSyncRunner is the production implementation.
type prodSyncRunner struct{}

func (prodSyncRunner) Run(ctx context.Context, opts syncmon.Options) error {
	return syncmon.Run(ctx, opts)
}

// syncCoreOpts holds options for the sync core logic.
type syncCoreOpts struct {
	rpc          string
	remote       string
	logPath      string
	window       int
	compact      bool
	interval     time.Duration
	stuckTimeout time.Duration
	skipFinal    bool
	quiet        bool
	debug        bool
}

// runSyncCore contains the testable sync logic.
func runSyncCore(ctx context.Context, runner SyncRunner, opts syncCoreOpts, output io.Writer) error {
	stuckTimeout := opts.stuckTimeout
	if stuckTimeout <= 0 {
		if envTimeout := os.Getenv("PNM_SYNC_STUCK_TIMEOUT"); envTimeout != "" {
			if parsed, err := time.ParseDuration(envTimeout); err == nil {
				stuckTimeout = parsed
			}
		}
	}
	if err := runner.Run(ctx, syncmon.Options{
		LocalRPC:     opts.rpc,
		RemoteRPC:    opts.remote,
		LogPath:      opts.logPath,
		Window:       opts.window,
		Compact:      opts.compact,
		Out:          output,
		Interval:     opts.interval,
		Quiet:        opts.quiet,
		Debug:        opts.debug,
		StuckTimeout: stuckTimeout,
	}); err != nil {
		if errors.Is(err, syncmon.ErrSyncStuck) {
			return exitcodes.NewError(exitcodes.SyncStuck, err.Error())
		}
		return err
	}
	if !opts.skipFinal {
		if opts.quiet {
			fmt.Fprintln(output, "  Sync complete.")
		} else {
			fmt.Fprintln(output, "  \u2713 Sync complete! Node is fully synced.")
		}
	}
	return nil
}

func init() {
	var syncCompact bool
	var syncWindow int
	var syncRPC string
	var syncRemote string
	var syncSkipFinal bool
	var syncInterval time.Duration
	var syncStuckTimeout time.Duration

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Monitor sync progress",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadCfg()
			if syncRPC == "" {
				syncRPC = cfg.RPCLocal
			}
			if syncRemote == "" {
				syncRemote = cfg.RemoteRPCURL()
			}
			sup := newSupervisor(cfg.HomeDir)
			if err := checkNodeRunning(sup); err != nil {
				return err
			}
			return runSyncCore(cmd.Context(), prodSyncRunner{}, syncCoreOpts{
				rpc:          syncRPC,
				remote:       syncRemote,
				logPath:      sup.LogPath(),
				window:       syncWindow,
				compact:      syncCompact,
				interval:     syncInterval,
				stuckTimeout: syncStuckTimeout,
				skipFinal:    syncSkipFinal,
				quiet:        flagQuiet,
				debug:        flagDebug,
			}, cmd.OutOrStdout())
		},
	}
	syncCmd.Flags().BoolVar(&syncCompact, "compact", false, "Compact output")
	syncCmd.Flags().IntVar(&syncWindow, "window", 30, "Moving average window (headers)")
	syncCmd.Flags().StringVar(&syncRPC, "rpc", "", "Local RPC base (http[s]://host:port)")
	syncCmd.Flags().StringVar(&syncRemote, "remote", "", "Remote RPC base")
	syncCmd.Flags().DurationVar(&syncInterval, "interval", 120*time.Millisecond, "Update interval (e.g. 1s, 2s)")
	syncCmd.Flags().BoolVar(&syncSkipFinal, "skip-final-message", false, "Suppress completion message (for automation)")
	syncCmd.Flags().DurationVar(&syncStuckTimeout, "stuck-timeout", 0, "Stuck detection timeout (e.g. 2m, 5m). 0 uses default or PNM_SYNC_STUCK_TIMEOUT")
	rootCmd.AddCommand(syncCmd)
}
