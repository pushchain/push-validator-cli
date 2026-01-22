package main

import (
	"fmt"
	"os"

	"github.com/pushchain/push-validator-cli/internal/snapshot"
	"github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	var snapshotURL string

	snapshotCmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Snapshot management commands",
		Long:  `Commands for managing blockchain snapshots, including downloading and verifying.`,
	}

	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "Download blockchain snapshot to cache",
		Long: `Download a blockchain snapshot to the local cache.

This command downloads a compressed snapshot (~6-7GB) and verifies its checksum.
The snapshot is cached locally and can be extracted later using 'snapshot extract'.

Caching behavior:
- Downloaded snapshots are cached to ~/.pchain/snapshot-cache/
- Before downloading, compares remote checksum with cached version
- If checksums match, skips download (cache is valid)
- If checksums differ (new snapshot on server), downloads and updates cache
- Use --no-cache to force a fresh download

Examples:
  push-validator snapshot download
  push-validator snapshot download --no-cache
  push-validator snapshot download --snapshot-url https://custom-snapshot-server.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadCfg()

			// Use config snapshot URL if not specified
			if snapshotURL == "" {
				snapshotURL = cfg.SnapshotURL
			}
			if snapshotURL == "" {
				snapshotURL = snapshot.DefaultSnapshotURL
			}

			noCache, _ := cmd.Flags().GetBool("no-cache")

			if flagOutput != "json" {
				dim, reset := "\033[2m", "\033[0m"
				if os.Getenv("NO_COLOR") != "" {
					dim, reset = "", ""
				}
				fmt.Printf("  %s%-12s %s%s\n", dim, "Source:", ui.ShortenPath(snapshotURL), reset)
				fmt.Printf("  %s%-12s %s%s\n", dim, "Cache:", ui.ShortenPath(cfg.HomeDir+"/"+snapshot.CacheDir), reset)
			}

			// Create progress bar callback
			var bar *ui.ProgressBar
			progressCallback := func(phase snapshot.ProgressPhase, current, total int64, message string) {
				if flagOutput == "json" {
					return
				}
				switch phase {
				case snapshot.PhaseCache:
					if message != "" {
						fmt.Printf("  → %s\n", message)
					}
				case snapshot.PhaseDownload:
					if bar == nil && total > 0 {
						bar = ui.NewProgressBar(os.Stdout, total)
						bar.SetIndent("  ") // 2-space indent to align with Source/Cache
					}
					if bar != nil {
						bar.Update(current)
					} else if message != "" {
						fmt.Printf("  → %s\n", message)
					}
				case snapshot.PhaseVerify:
					if bar != nil {
						bar.Finish()
						bar = nil
					}
					if message != "" {
						fmt.Printf("  → %s\n", message)
					}
				}
			}

			svc := snapshot.New()
			if err := svc.Download(cmd.Context(), snapshot.Options{
				SnapshotURL: snapshotURL,
				HomeDir:     cfg.HomeDir,
				Progress:    progressCallback,
				NoCache:     noCache,
			}); err != nil {
				return fmt.Errorf("snapshot download failed: %w", err)
			}

			return nil
		},
	}

	downloadCmd.Flags().StringVar(&snapshotURL, "snapshot-url", "", "Snapshot download URL (default: from config)")
	downloadCmd.Flags().Bool("no-cache", false, "Force fresh download, bypass cache check")

	// Extract command
	extractCmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract cached snapshot to data directory",
		Long: `Extract the cached blockchain snapshot to the node's data directory.

This command extracts a previously downloaded snapshot from the cache directly
to the data directory. Run this after 'pchaind init' to apply the snapshot.

The extraction preserves priv_validator_state.json if it exists.

Examples:
  push-validator snapshot extract
  push-validator snapshot extract --target /custom/data/path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadCfg()
			p := ui.NewPrinter(flagOutput)

			targetDir, _ := cmd.Flags().GetString("target")
			if targetDir == "" {
				targetDir = cfg.HomeDir + "/data"
			}

			// Check if already extracted (unless force flag)
			force, _ := cmd.Flags().GetBool("force")
			if !force && snapshot.IsSnapshotPresent(cfg.HomeDir) {
				p.Success("Snapshot already extracted, skipping")
				if flagOutput != "json" {
					fmt.Println("  Use --force to re-extract")
				}
				return nil
			}

			if flagOutput != "json" {
				dim, reset := "\033[2m", "\033[0m"
				if os.Getenv("NO_COLOR") != "" {
					dim, reset = "", ""
				}
				fmt.Printf("  %s%-12s %s%s\n", dim, "Cache:", ui.ShortenPath(cfg.HomeDir+"/"+snapshot.CacheDir), reset)
				fmt.Printf("  %s%-12s %s%s\n", dim, "Destination:", ui.ShortenPath(targetDir), reset)
			}

			// Create progress callback
			progressCallback := func(phase snapshot.ProgressPhase, current, total int64, message string) {
				if flagOutput == "json" {
					return
				}
				if phase == snapshot.PhaseExtract && message != "" {
					fmt.Printf("\r  → Extracting: %-60s", truncate(message, 60))
				}
			}

			svc := snapshot.New()
			if err := svc.Extract(cmd.Context(), snapshot.ExtractOptions{
				HomeDir:   cfg.HomeDir,
				TargetDir: targetDir,
				Progress:  progressCallback,
			}); err != nil {
				return fmt.Errorf("snapshot extract failed: %w", err)
			}

			if flagOutput != "json" {
				fmt.Println() // Clear extraction line
			}
			return nil
		},
	}

	extractCmd.Flags().String("target", "", "Target directory for extraction (default: ~/.pchain/data)")
	extractCmd.Flags().Bool("force", false, "Force extraction even if snapshot already exists")

	snapshotCmd.AddCommand(downloadCmd)
	snapshotCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(snapshotCmd)
}

// truncate shortens a string to max length, adding "..." if truncated
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
