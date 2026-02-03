package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

func TestHandleVote_Success(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true
	flagYes = true // Skip confirmation
	flagNonInteractive = true

	d := &Deps{
		Cfg: config.Config{
			GenesisDomain:   "test.rpc.push.org",
			HomeDir:         "/tmp/test",
			KeyringBackend:  "test",
			ChainID:         "push_42101-1",
		},
		Fetcher: &mockFetcher{
			proposals: validator.ProposalList{
				Proposals: []validator.Proposal{
					{ID: "1", Title: "Test Proposal", Status: "VOTING", VotingEnd: "2024-12-31T23:59:59Z"},
				},
				Total: 1,
			},
			myValidator: validator.MyValidatorInfo{
				IsValidator: true,
				Address:     "pushvaloper1abc123",
			},
		},
		Validator: &mockValidator{
			voteResult: "ABCD1234TXHASH",
		},
		Runner: &mockRunner{
			outputs: map[string][]byte{},
		},
	}

	err := handleVote(d, "1", "yes")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestHandleVote_Success_JSONOutput(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "json"
	flagYes = true
	flagNonInteractive = true

	d := &Deps{
		Cfg: config.Config{
			GenesisDomain:   "test.rpc.push.org",
			HomeDir:         "/tmp/test",
			KeyringBackend:  "test",
		},
		Fetcher: &mockFetcher{
			proposals: validator.ProposalList{
				Proposals: []validator.Proposal{
					{ID: "1", Title: "Test Proposal", Status: "VOTING", VotingEnd: "2024-12-31T23:59:59Z"},
				},
				Total: 1,
			},
		},
		Validator: &mockValidator{
			voteResult: "TXHASH123",
		},
		Runner: &mockRunner{},
	}

	err := handleVote(d, "1", "yes")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestHandleVote_InvalidOption(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
	}()

	flagOutput = "text"
	flagNoColor = true

	d := &Deps{
		Cfg: config.Config{GenesisDomain: "test.rpc.push.org"},
	}

	// Test various invalid options
	invalidOptions := []string{"maybe", "yess", "noo", "invalid", "123", "YES!", ""}

	for _, opt := range invalidOptions {
		t.Run("invalid_"+opt, func(t *testing.T) {
			err := handleVote(d, "1", opt)
			if err == nil {
				t.Errorf("expected error for invalid option %q, got nil", opt)
			}
			if !strings.Contains(err.Error(), "invalid vote option") {
				t.Errorf("expected 'invalid vote option' error, got: %v", err)
			}
		})
	}
}

func TestHandleVote_ValidOptions(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true
	flagYes = true
	flagNonInteractive = true

	validOptions := []string{"yes", "no", "abstain", "no_with_veto", "YES", "NO", "ABSTAIN", "NO_WITH_VETO"}

	for _, opt := range validOptions {
		t.Run("valid_"+opt, func(t *testing.T) {
			d := &Deps{
				Cfg: config.Config{
					GenesisDomain:   "test.rpc.push.org",
					HomeDir:         "/tmp/test",
					KeyringBackend:  "test",
				},
				Fetcher: &mockFetcher{
					proposals: validator.ProposalList{
						Proposals: []validator.Proposal{
							{ID: "1", Title: "Test Proposal", Status: "VOTING", VotingEnd: "2024-12-31T23:59:59Z"},
						},
						Total: 1,
					},
				},
				Validator: &mockValidator{
					voteResult: "TXHASH",
				},
				Runner: &mockRunner{},
			}

			err := handleVote(d, "1", opt)
			if err != nil {
				t.Errorf("expected no error for valid option %q, got: %v", opt, err)
			}
		})
	}
}

func TestHandleVote_ProposalNotFound(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "text"
	flagNoColor = true
	flagYes = true
	flagNonInteractive = true

	d := &Deps{
		Cfg: config.Config{GenesisDomain: "test.rpc.push.org"},
		Fetcher: &mockFetcher{
			proposals: validator.ProposalList{
				Proposals: []validator.Proposal{
					{ID: "1", Title: "Test Proposal", Status: "VOTING"},
				},
				Total: 1,
			},
		},
	}

	err := handleVote(d, "999", "yes") // Proposal 999 doesn't exist
	if err == nil {
		t.Fatal("expected error for non-existent proposal, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestHandleVote_ProposalNotInVotingPeriod(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "text"
	flagNoColor = true
	flagYes = true
	flagNonInteractive = true

	tests := []struct {
		name   string
		status string
	}{
		{"passed", "PASSED"},
		{"rejected", "REJECTED"},
		{"deposit", "DEPOSIT"},
		{"failed", "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Deps{
				Cfg: config.Config{GenesisDomain: "test.rpc.push.org"},
				Fetcher: &mockFetcher{
					proposals: validator.ProposalList{
						Proposals: []validator.Proposal{
							{ID: "1", Title: "Test Proposal", Status: tt.status},
						},
						Total: 1,
					},
				},
			}

			err := handleVote(d, "1", "yes")
			if err == nil {
				t.Errorf("expected error for %s proposal, got nil", tt.status)
			}

			if !strings.Contains(err.Error(), "not in voting period") {
				t.Errorf("expected 'not in voting period' error, got: %v", err)
			}
		})
	}
}

func TestHandleVote_FetchProposalsError(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
	}()

	flagOutput = "text"
	flagNoColor = true

	d := &Deps{
		Cfg: config.Config{GenesisDomain: "test.rpc.push.org"},
		Fetcher: &mockFetcher{
			proposalsErr: errors.New("network error"),
		},
	}

	err := handleVote(d, "1", "yes")
	if err == nil {
		t.Fatal("expected error when fetching proposals fails, got nil")
	}

	if !strings.Contains(err.Error(), "fetch proposals") {
		t.Errorf("expected fetch error, got: %v", err)
	}
}

func TestHandleVote_VoteTransactionError(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "text"
	flagNoColor = true
	flagYes = true
	flagNonInteractive = true

	d := &Deps{
		Cfg: config.Config{
			GenesisDomain:   "test.rpc.push.org",
			HomeDir:         "/tmp/test",
			KeyringBackend:  "test",
		},
		Fetcher: &mockFetcher{
			proposals: validator.ProposalList{
				Proposals: []validator.Proposal{
					{ID: "1", Title: "Test Proposal", Status: "VOTING", VotingEnd: "2024-12-31T23:59:59Z"},
				},
				Total: 1,
			},
		},
		Validator: &mockValidator{
			voteErr: errors.New("insufficient funds for gas"),
		},
		Runner: &mockRunner{},
	}

	err := handleVote(d, "1", "yes")
	if err == nil {
		t.Fatal("expected error when vote transaction fails, got nil")
	}

	if !strings.Contains(err.Error(), "vote failed") {
		t.Errorf("expected 'vote failed' error, got: %v", err)
	}
}

func TestHandleVote_AlreadyVotedError(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "text"
	flagNoColor = true
	flagYes = true
	flagNonInteractive = true

	d := &Deps{
		Cfg: config.Config{
			GenesisDomain:   "test.rpc.push.org",
			HomeDir:         "/tmp/test",
			KeyringBackend:  "test",
		},
		Fetcher: &mockFetcher{
			proposals: validator.ProposalList{
				Proposals: []validator.Proposal{
					{ID: "1", Title: "Test Proposal", Status: "VOTING", VotingEnd: "2024-12-31T23:59:59Z"},
				},
				Total: 1,
			},
		},
		Validator: &mockValidator{
			voteErr: errors.New("You have already voted on this proposal."),
		},
		Runner: &mockRunner{},
	}

	err := handleVote(d, "1", "yes")
	if err == nil {
		t.Fatal("expected error when already voted, got nil")
	}

	// Error is now generic since detailed message is shown in UI
	if !strings.Contains(err.Error(), "vote failed") {
		t.Errorf("expected 'vote failed' error, got: %v", err)
	}
}

func TestHandleVote_JSONOutput_Error(t *testing.T) {
	origOutput := flagOutput
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "json"
	flagYes = true
	flagNonInteractive = true

	d := &Deps{
		Cfg: config.Config{GenesisDomain: "test.rpc.push.org"},
		Fetcher: &mockFetcher{
			proposalsErr: errors.New("network error"),
		},
	}

	err := handleVote(d, "1", "yes")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHandleVote_EmptyProposalID(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
	}()

	flagOutput = "text"
	flagNoColor = true

	d := &Deps{
		Cfg: config.Config{GenesisDomain: "test.rpc.push.org"},
		Fetcher: &mockFetcher{
			proposals: validator.ProposalList{
				Proposals: []validator.Proposal{
					{ID: "1", Title: "Test Proposal", Status: "VOTING"},
				},
				Total: 1,
			},
		},
	}

	// Empty proposal ID should not match any proposal
	err := handleVote(d, "", "yes")
	if err == nil {
		t.Fatal("expected error for empty proposal ID, got nil")
	}
}

func TestGetInteractiveReader(t *testing.T) {
	// This is a simple test to ensure the function doesn't panic
	reader := getInteractiveReader()
	if reader == nil {
		t.Error("expected non-nil reader")
	}
}

// Test the Vote service method error messages
// Note: Detailed error messages are now displayed in UI output, returned error is generic "vote failed"
func TestVoteErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{"insufficient_funds", errors.New("insufficient funds for gas"), "vote failed"},
		{"already_voted", errors.New("You have already voted on this proposal."), "vote failed"},
		{"proposal_not_found", errors.New("Proposal not found. Check that the proposal ID is correct."), "vote failed"},
		{"not_voting_period", errors.New("Proposal is not in voting period. You can only vote on active proposals."), "vote failed"},
	}

	origOutput := flagOutput
	origNoColor := flagNoColor
	origYes := flagYes
	origNonInteractive := flagNonInteractive
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagYes = origYes
		flagNonInteractive = origNonInteractive
	}()

	flagOutput = "text"
	flagNoColor = true
	flagYes = true
	flagNonInteractive = true

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Deps{
				Cfg: config.Config{
					GenesisDomain:   "test.rpc.push.org",
					HomeDir:         "/tmp/test",
					KeyringBackend:  "test",
				},
				Fetcher: &mockFetcher{
					proposals: validator.ProposalList{
						Proposals: []validator.Proposal{
							{ID: "1", Title: "Test Proposal", Status: "VOTING", VotingEnd: "2024-12-31T23:59:59Z"},
						},
						Total: 1,
					},
				},
				Validator: &mockValidator{
					voteErr: tt.err,
				},
				Runner: &mockRunner{},
			}

			err := handleVote(d, "1", "yes")
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("expected error to contain %q, got: %v", tt.contains, err)
			}
		})
	}
}
