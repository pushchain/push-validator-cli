package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pushchain/push-validator-cli/internal/validator"
)

func init() {
	voteCmd := &cobra.Command{
		Use:   "vote <proposal-id> <option>",
		Short: "Vote on a governance proposal",
		Long: `Vote on an active governance proposal.

Options:
  yes           - Vote in favor of the proposal
  no            - Vote against the proposal
  abstain       - Abstain from voting (neither yes nor no)
  no_with_veto  - Vote against with veto (counts towards veto threshold)

Examples:
  push-validator vote 1 yes
  push-validator vote 1 no
  push-validator vote 1 abstain
  push-validator vote 1 no_with_veto`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing proposal ID and vote option\n\nUsage: push-validator vote <proposal-id> <option>\nExample: push-validator vote 1 yes")
			}
			if len(args) < 2 {
				return fmt.Errorf("missing vote option\n\nUsage: push-validator vote %s <option>\nExample: push-validator vote %s yes\n\nValid options: yes, no, abstain, no_with_veto", args[0], args[0])
			}
			if len(args) > 2 {
				return fmt.Errorf("too many arguments\n\nUsage: push-validator vote <proposal-id> <option>\nExample: push-validator vote 1 yes")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleVote(newDeps(), args[0], args[1])
		},
	}
	rootCmd.AddCommand(voteCmd)
}

func handleVote(d *Deps, proposalID, option string) error {
	p := getPrinter()
	cfg := d.Cfg

	// Validate vote option early
	validOptions := map[string]bool{
		"yes":          true,
		"no":           true,
		"abstain":      true,
		"no_with_veto": true,
	}
	optionLower := strings.ToLower(option)
	if !validOptions[optionLower] {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": fmt.Sprintf("invalid vote option: %s", option)})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Invalid vote option: " + option))
			fmt.Println()
			fmt.Println(p.Colors.Info("Valid options: yes, no, abstain, no_with_veto"))
			fmt.Println()
		}
		return silentErr{fmt.Errorf("invalid vote option: %s", option)}
	}

	// Step 1: Verify proposal exists and is in voting period
	if flagOutput != "json" {
		fmt.Println()
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("🔍")+" Checking proposal status..."))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	proposals, err := d.Fetcher.GetProposals(ctx, cfg)
	cancel()

	if err != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": "failed to fetch proposals"})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Failed to fetch proposals"))
			fmt.Println()
			fmt.Println(p.Colors.Info("Check your network connection and try again"))
			fmt.Println()
		}
		return silentErr{fmt.Errorf("failed to fetch proposals")}
	}

	// Find the proposal
	var targetProposal *validator.Proposal
	for i, prop := range proposals.Proposals {
		if prop.ID == proposalID {
			targetProposal = &proposals.Proposals[i]
			break
		}
	}

	if targetProposal == nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": fmt.Sprintf("proposal %s not found", proposalID)})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Proposal " + proposalID + " not found"))
			fmt.Println()
			fmt.Println(p.Colors.Info("Use 'push-validator proposals' to list available proposals"))
			fmt.Println()
		}
		return silentErr{fmt.Errorf("proposal %s not found", proposalID)}
	}

	// Check if proposal is in voting period
	if targetProposal.Status != "VOTING" {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": fmt.Sprintf("proposal %s is not in voting period (status: %s)", proposalID, targetProposal.Status)})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Warning(p.Colors.Emoji("⚠️") + " Proposal " + proposalID + " is not in voting period"))
			fmt.Println()
			fmt.Printf("Current status: %s\n", targetProposal.Status)
			fmt.Println()
			fmt.Println(p.Colors.Info("You can only vote on proposals in VOTING status"))
			fmt.Println()
		}
		return silentErr{fmt.Errorf("proposal %s is not in voting period", proposalID)}
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	// Step 2: Display proposal info and confirm
	if flagOutput != "json" && !flagYes && !flagNonInteractive {
		fmt.Println()
		fmt.Println(p.Colors.SubHeader("Proposal Details"))
		fmt.Println(p.Colors.Separator(50))
		p.KeyValueLine("ID", targetProposal.ID, "")
		p.KeyValueLine("Title", targetProposal.Title, "")
		p.KeyValueLine("Status", targetProposal.Status, "yellow")
		if targetProposal.VotingEnd != "" {
			if t, err := time.Parse(time.RFC3339, targetProposal.VotingEnd); err == nil {
				p.KeyValueLine("Voting Ends", t.Format("2006-01-02 15:04:05"), "")
			}
		}
		fmt.Println()
		fmt.Printf("Your vote: %s\n", p.Colors.Apply(p.Colors.Theme.Value, strings.ToUpper(optionLower)))
		fmt.Println()

		// Confirm vote
		reader := getInteractiveReader()
		fmt.Print("Confirm vote? [y/N]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println()
			fmt.Println(p.Colors.Info("Vote cancelled"))
			return nil
		}
		fmt.Println()
	}

	// Step 3: Get key name
	defaultKeyName := getenvDefault("KEY_NAME", "validator-key")
	keyName := defaultKeyName

	// Try to get key from my validator info
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	myVal, valErr := d.Fetcher.GetMyValidator(ctx2, cfg)
	cancel2()

	if valErr == nil && myVal.IsValidator && myVal.Address != "" {
		// Try to find key by validator address
		addrCtx, addrCancel := context.WithTimeout(context.Background(), 10*time.Second)
		accountAddr, convErr := convertValidatorToAccountAddress(addrCtx, myVal.Address, d.Runner)
		addrCancel()
		if convErr == nil {
			keyCtx, keyCancel := context.WithTimeout(context.Background(), 10*time.Second)
			foundKey, findErr := findKeyNameByAddress(keyCtx, cfg, accountAddr, d.Runner)
			keyCancel()
			if findErr == nil {
				keyName = foundKey
			}
		}
	}

	// Prompt for key if needed and interactive
	if flagOutput != "json" && !flagNonInteractive && keyName == defaultKeyName && os.Getenv("KEY_NAME") == "" {
		reader := getInteractiveReader()
		fmt.Printf("Enter key name for voting [%s]: ", defaultKeyName)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			keyName = input
		}
		fmt.Println()
	}

	if flagOutput != "json" {
		fmt.Printf("%s Using key: %s\n", p.Colors.Emoji("🔑"), keyName)
		fmt.Println()
	}

	// Step 4: Submit vote
	if flagOutput != "json" {
		fmt.Print(p.Colors.Apply(p.Colors.Theme.Prompt, p.Colors.Emoji("📤")+" Submitting vote..."))
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel3()

	txHash, err := d.Validator.Vote(ctx3, validator.VoteArgs{
		ProposalID: proposalID,
		Option:     optionLower,
		KeyName:    keyName,
	})

	if err != nil {
		if flagOutput == "json" {
			getPrinter().JSON(map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Println()
			fmt.Println(p.Colors.Error(p.Colors.Emoji("❌") + " Vote failed"))
			fmt.Println()
			fmt.Printf("Error: %v\n", err)
			fmt.Println()
		}
		return silentErr{fmt.Errorf("vote failed")}
	}

	if flagOutput != "json" {
		fmt.Println(" " + p.Colors.Success(p.Colors.Emoji("✓")))
	}

	// Success output
	if flagOutput == "json" {
		getPrinter().JSON(map[string]any{
			"ok":          true,
			"txhash":      txHash,
			"proposal_id": proposalID,
			"vote":        optionLower,
		})
	} else {
		fmt.Println()
		p.Success(p.Colors.Emoji("✅") + " Vote submitted successfully!")
		fmt.Println()
		p.KeyValueLine("Proposal", fmt.Sprintf("#%s - %s", targetProposal.ID, targetProposal.Title), "")
		p.KeyValueLine("Vote", strings.ToUpper(optionLower), "green")
		p.KeyValueLine("Transaction Hash", txHash, "green")
		fmt.Println()
	}

	return nil
}

// getInteractiveReader returns a reader for interactive input, handling pipes
func getInteractiveReader() *bufio.Reader {
	savedStdin := os.Stdin
	if !term.IsTerminal(int(savedStdin.Fd())) {
		if tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0); err == nil {
			return bufio.NewReader(tty)
		}
	}
	return bufio.NewReader(os.Stdin)
}
