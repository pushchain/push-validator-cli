package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	ui "github.com/pushchain/push-validator-cli/internal/ui"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

var flagProposalStatus string

func init() {
	proposalsCmd := &cobra.Command{
		Use:   "proposals",
		Short: "List governance proposals",
		Long:  "List all governance proposals on the Push Chain, optionally filtered by status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleProposals(newDeps(), flagOutput == "json")
		},
	}
	proposalsCmd.Flags().StringVar(&flagProposalStatus, "status", "", "Filter by status: voting, passed, rejected, deposit")
	rootCmd.AddCommand(proposalsCmd)
}

func handleProposals(d *Deps, jsonOut bool) error {
	cfg := d.Cfg

	// For JSON output, query raw data directly
	if jsonOut {
		remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		args := []string{"query", "gov", "proposals", "--node", remote, "-o", "json"}
		if flagProposalStatus != "" {
			args = append(args, "--status", mapStatusFlag(flagProposalStatus))
		}

		output, err := d.Runner.Run(ctx, findPchaind(), args...)
		if err != nil {
			p := getPrinter()
			if ctx.Err() == context.DeadlineExceeded {
				p.JSON(map[string]any{"ok": false, "error": "timeout connecting to network"})
				return silentErr{fmt.Errorf("timeout")}
			}
			p.JSON(map[string]any{"ok": false, "error": "failed to fetch proposals"})
			return silentErr{fmt.Errorf("failed to fetch proposals")}
		}
		fmt.Println(string(output))
		return nil
	}

	// For table output, use cached fetcher
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	propList, err := d.Fetcher.GetProposals(ctx, cfg)
	if err != nil {
		c := ui.NewColorConfig()
		fmt.Println()
		fmt.Println(c.Error(c.Emoji("❌") + " Failed to fetch proposals"))
		fmt.Println()
		fmt.Println(c.Info("Check your network connection and try again"))
		fmt.Println()
		return silentErr{fmt.Errorf("failed to fetch proposals")}
	}

	if propList.Total == 0 {
		c := ui.NewColorConfig()
		fmt.Println()
		fmt.Println(c.Header(" 📜 Governance Proposals "))
		fmt.Println(c.Info("No proposals found"))
		return nil
	}

	// Filter by status if specified
	filtered := propList.Proposals
	if flagProposalStatus != "" {
		statusFilter := strings.ToUpper(flagProposalStatus)
		filtered = make([]validator.Proposal, 0)
		for _, p := range propList.Proposals {
			if strings.EqualFold(p.Status, statusFilter) {
				filtered = append(filtered, p)
			}
		}
	}

	// Sort by voting end date descending (most recent first), then by ID
	sort.Slice(filtered, func(i, j int) bool {
		// Parse voting end times
		var timeI, timeJ time.Time
		if filtered[i].VotingEnd != "" {
			timeI, _ = time.Parse(time.RFC3339, filtered[i].VotingEnd)
		}
		if filtered[j].VotingEnd != "" {
			timeJ, _ = time.Parse(time.RFC3339, filtered[j].VotingEnd)
		}

		// If both have dates, sort by date descending
		if !timeI.IsZero() && !timeJ.IsZero() {
			return timeI.After(timeJ)
		}
		// If only one has a date, prioritize the one with a date
		if !timeI.IsZero() {
			return true
		}
		if !timeJ.IsZero() {
			return false
		}
		// If neither has a date, sort by ID descending
		idI, _ := strconv.Atoi(filtered[i].ID)
		idJ, _ := strconv.Atoi(filtered[j].ID)
		return idI > idJ
	})

	c := ui.NewColorConfig()
	fmt.Println()
	fmt.Println(c.Header(" 📜 Governance Proposals "))

	if len(filtered) == 0 {
		fmt.Println(c.Info("No proposals match the filter"))
		return nil
	}

	headers := []string{"ID", "TITLE", "STATUS", "VOTING ENDS"}
	rows := make([][]string, 0, len(filtered))

	for _, p := range filtered {
		// Truncate title if too long
		title := p.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}

		// Format voting end time
		votingEnd := "—"
		if p.VotingEnd != "" {
			if t, err := time.Parse(time.RFC3339, p.VotingEnd); err == nil {
				votingEnd = t.Format("2006-01-02 15:04")
			}
		}

		// Color status based on state
		status := p.Status
		switch p.Status {
		case "VOTING":
			status = c.Warning(p.Status)
		case "PASSED":
			status = c.Success(p.Status)
		case "REJECTED", "FAILED":
			status = c.Error(p.Status)
		case "DEPOSIT":
			status = c.Info(p.Status)
		}

		rows = append(rows, []string{
			p.ID,
			title,
			status,
			votingEnd,
		})
	}

	fmt.Print(ui.Table(c, headers, rows, nil))
	fmt.Printf("Total Proposals: %d\n", len(filtered))

	// Show tip about voting if there are voting proposals
	hasVoting := false
	for _, p := range filtered {
		if p.Status == "VOTING" {
			hasVoting = true
			break
		}
	}
	if hasVoting {
		fmt.Println(c.Info("💡 Tip: Use 'push-validator vote <id> <yes|no|abstain|no_with_veto>' to vote"))
	}

	return nil
}

// mapStatusFlag maps user-friendly status names to chain status values
func mapStatusFlag(status string) string {
	switch strings.ToLower(status) {
	case "voting":
		return "voting_period"
	case "passed":
		return "passed"
	case "rejected":
		return "rejected"
	case "deposit":
		return "deposit_period"
	default:
		return status
	}
}
