package validator

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcutil/bech32"
	"github.com/pushchain/push-validator-cli/internal/config"
)

// Bech32ToHex converts a bech32 address (push1..., pushvaloper1...) to EVM hex format (0x...)
// This is a pure Go implementation that doesn't require subprocess calls.
func Bech32ToHex(addr string) string {
	if addr == "" {
		return "—"
	}

	// Decode bech32 address
	_, data, err := bech32.Decode(addr)
	if err != nil {
		return "—"
	}

	// Convert 5-bit groups to 8-bit bytes
	converted, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "—"
	}

	return "0x" + strings.ToUpper(hex.EncodeToString(converted))
}

// commandContext creates an exec.CommandContext with DYLD_LIBRARY_PATH set for macOS
// to find libwasmvm.dylib in the same directory as the binary
func commandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
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

	return cmd
}

// resolvePchaindBin finds pchaind binary in PATH or cosmovisor directory.
// Prefers cosmovisor binaries to ensure libwasmvm.dylib compatibility.
func resolvePchaindBin(homeDir string) (string, error) {
	// Check cosmovisor genesis directory first (has matching libwasmvm.dylib)
	cosmovisorPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "pchaind")
	if _, err := os.Stat(cosmovisorPath); err == nil {
		return cosmovisorPath, nil
	}
	// Check cosmovisor current directory
	currentPath := filepath.Join(homeDir, "cosmovisor", "current", "bin", "pchaind")
	if _, err := os.Stat(currentPath); err == nil {
		return currentPath, nil
	}
	// Fallback to PATH (may have dylib compatibility issues)
	if bin, err := exec.LookPath("pchaind"); err == nil {
		return bin, nil
	}
	return "", fmt.Errorf("pchaind not found in PATH or %s", filepath.Join(homeDir, "cosmovisor"))
}



// rewardsCacheEntry holds cached rewards data with timestamp
type rewardsCacheEntry struct {
	commission  string
	outstanding string
	fetchedAt   time.Time
}

// Fetcher handles validator data fetching with caching
type Fetcher struct {
	mu sync.Mutex

	// All validators cache
	allValidators     ValidatorList
	allValidatorsTime time.Time

	// My validator cache
	myValidator     MyValidatorInfo
	myValidatorTime time.Time

	// Rewards cache (per validator address)
	rewardsCache map[string]rewardsCacheEntry
	rewardsTTL   time.Duration

	// Proposals cache
	proposals     ProposalList
	proposalsTime time.Time

	cacheTTL time.Duration
}

// NewFetcher creates a new validator fetcher with 30s cache
func NewFetcher() *Fetcher {
	return &Fetcher{
		cacheTTL:     30 * time.Second,
		rewardsTTL:   30 * time.Second,
		rewardsCache: make(map[string]rewardsCacheEntry),
	}
}

// GetAllValidators fetches all validators with 30s caching
func (f *Fetcher) GetAllValidators(ctx context.Context, cfg config.Config) (ValidatorList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Force fetch on first call (cache is zero-initialized)
	if f.allValidatorsTime.IsZero() {
		list, err := f.fetchAllValidators(ctx, cfg)
		if err != nil {
			return ValidatorList{}, err
		}
		f.allValidators = list
		f.allValidatorsTime = time.Now()
		return list, nil
	}

	// Return cached if still valid
	if time.Since(f.allValidatorsTime) < f.cacheTTL && f.allValidators.Total > 0 {
		return f.allValidators, nil
	}

	// Fetch fresh data
	list, err := f.fetchAllValidators(ctx, cfg)
	if err != nil {
		// Return stale cache if available
		if f.allValidators.Total > 0 {
			return f.allValidators, nil
		}
		return ValidatorList{}, err
	}

	// Update cache
	f.allValidators = list
	f.allValidatorsTime = time.Now()
	return list, nil
}

// GetMyValidator fetches current node's validator status with 30s caching
func (f *Fetcher) GetMyValidator(ctx context.Context, cfg config.Config) (MyValidatorInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Force fetch on first call (cache is zero-initialized)
	if f.myValidatorTime.IsZero() {
		myVal, err := f.fetchMyValidator(ctx, cfg)
		if err != nil {
			// IMPORTANT: Set cache time even on error to prevent infinite retry loops
			f.myValidatorTime = time.Now()
			return MyValidatorInfo{IsValidator: false}, err
		}
		f.myValidator = myVal
		f.myValidatorTime = time.Now()
		return myVal, nil
	}

	// Return cached if still valid
	if time.Since(f.myValidatorTime) < f.cacheTTL {
		return f.myValidator, nil
	}

	// Fetch fresh data
	myVal, err := f.fetchMyValidator(ctx, cfg)
	if err != nil {
		// Return stale cache if available
		if f.myValidator.Address != "" || !f.myValidatorTime.IsZero() {
			return f.myValidator, nil
		}
		// Set cache time to retry on next refresh
		f.myValidatorTime = time.Now()
		return MyValidatorInfo{IsValidator: false}, err
	}

	// Update cache
	f.myValidator = myVal
	f.myValidatorTime = time.Now()
	return myVal, nil
}

// fetchAllValidators queries all validators from the network
func (f *Fetcher) fetchAllValidators(ctx context.Context, cfg config.Config) (ValidatorList, error) {
	bin, err := resolvePchaindBin(cfg.HomeDir)
	if err != nil {
		return ValidatorList{}, fmt.Errorf("pchaind not found: %w", err)
	}

	remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)
	cmd := commandContext(ctx, bin, "query", "staking", "validators", "--node", remote, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return ValidatorList{}, fmt.Errorf("query validators failed: %w", err)
	}

	var result struct {
		Validators []struct {
			Description struct {
				Moniker string `json:"moniker"`
			} `json:"description"`
			OperatorAddress string `json:"operator_address"`
			Status          string `json:"status"`
			Tokens          string `json:"tokens"`
			Commission      struct {
				CommissionRates struct {
					Rate string `json:"rate"`
				} `json:"commission_rates"`
			} `json:"commission"`
			Jailed bool `json:"jailed"`
		} `json:"validators"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return ValidatorList{}, fmt.Errorf("parse validators failed: %w", err)
	}

	validators := make([]ValidatorInfo, 0, len(result.Validators))
	for _, v := range result.Validators {
		moniker := v.Description.Moniker
		if moniker == "" {
			moniker = "unknown"
		}

		status := parseStatus(v.Status)

		var votingPower int64
		if v.Tokens != "" {
			if tokens, err := strconv.ParseFloat(v.Tokens, 64); err == nil {
				votingPower = int64(tokens / 1e18)
			}
		}

		commission := "0%"
		if v.Commission.CommissionRates.Rate != "" {
			if rate, err := strconv.ParseFloat(v.Commission.CommissionRates.Rate, 64); err == nil {
				if rate > 1 {
					rate = rate / 1e18
				}
				commission = fmt.Sprintf("%.0f%%", rate*100)
			}
		}

		validators = append(validators, ValidatorInfo{
			OperatorAddress: v.OperatorAddress,
			Moniker:         moniker,
			Status:          status,
			Tokens:          v.Tokens,
			VotingPower:     votingPower,
			Commission:      commission,
			Jailed:          v.Jailed,
		})
	}

	return ValidatorList{
		Validators: validators,
		Total:      len(validators),
	}, nil
}

// fetchMyValidator fetches the current node's validator info by comparing consensus pubkeys
func (f *Fetcher) fetchMyValidator(ctx context.Context, cfg config.Config) (MyValidatorInfo, error) {
	bin, err := resolvePchaindBin(cfg.HomeDir)
	if err != nil {
		return MyValidatorInfo{IsValidator: false}, nil
	}

	// Get local consensus pubkey using 'tendermint show-validator'
	showValCmd := commandContext(ctx, bin, "tendermint", "show-validator", "--home", cfg.HomeDir)
	pubkeyBytes, err := showValCmd.Output()
	if err != nil {
		// No validator key file exists
		return MyValidatorInfo{IsValidator: false}, nil
	}

	var localPubkey struct {
		Type string `json:"@type"`
		Key  string `json:"key"`
	}
	if err := json.Unmarshal(pubkeyBytes, &localPubkey); err != nil {
		return MyValidatorInfo{IsValidator: false}, nil
	}

	if localPubkey.Key == "" {
		return MyValidatorInfo{IsValidator: false}, nil
	}

	// Build the full pubkey JSON string for slashing info query
	fullPubkeyJSON := string(pubkeyBytes)

	// Get local node moniker from status (for conflict detection)
	var localMoniker string
	statusCmd := commandContext(ctx, bin, "status", "--node", cfg.RPCLocal)
	if statusOutput, err := statusCmd.Output(); err == nil {
		var statusData struct {
			NodeInfo struct {
				Moniker string `json:"moniker"`
			} `json:"NodeInfo"`
		}
		if json.Unmarshal(statusOutput, &statusData) == nil {
			localMoniker = statusData.NodeInfo.Moniker
		}
	}

	// Fetch all validators to match by consensus pubkey
	remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)
	queryCmd := commandContext(ctx, bin, "query", "staking", "validators", "--node", remote, "-o", "json")
	valsOutput, err := queryCmd.Output()
	if err != nil {
		return MyValidatorInfo{IsValidator: false}, err
	}

	var result struct {
		Validators []struct {
			OperatorAddress string `json:"operator_address"`
			Description     struct {
				Moniker string `json:"moniker"`
			} `json:"description"`
			ConsensusPubkey struct {
				Value string `json:"value"` // The base64 pubkey
			} `json:"consensus_pubkey"`
			Status     string `json:"status"`
			Tokens     string `json:"tokens"`
			Commission struct {
				CommissionRates struct {
					Rate string `json:"rate"`
				} `json:"commission_rates"`
			} `json:"commission"`
			Jailed bool `json:"jailed"`
		} `json:"validators"`
	}

	if err := json.Unmarshal(valsOutput, &result); err != nil {
		return MyValidatorInfo{IsValidator: false}, err
	}

	// Calculate total voting power
	var totalVotingPower int64
	for _, v := range result.Validators {
		if v.Tokens != "" {
			if tokens, err := strconv.ParseFloat(v.Tokens, 64); err == nil {
				totalVotingPower += int64(tokens / 1e18)
			}
		}
	}

	// Try to find validator by matching consensus pubkey
	var monikerConflict string
	for _, v := range result.Validators {
		// Check for moniker conflicts (different validator, same moniker)
		if localMoniker != "" && v.Description.Moniker == localMoniker &&
		   !strings.EqualFold(v.ConsensusPubkey.Value, localPubkey.Key) {
			monikerConflict = localMoniker
		}

		// Check if this validator matches our consensus pubkey
		if strings.EqualFold(v.ConsensusPubkey.Value, localPubkey.Key) {
			// Found our validator!
			status := parseStatus(v.Status)

			var votingPower int64
			if v.Tokens != "" {
				if tokens, err := strconv.ParseFloat(v.Tokens, 64); err == nil {
					votingPower = int64(tokens / 1e18)
				}
			}

			var votingPct float64
			if totalVotingPower > 0 {
				votingPct = float64(votingPower) / float64(totalVotingPower)
			}

			commission := "0%"
			if v.Commission.CommissionRates.Rate != "" {
				if rate, err := strconv.ParseFloat(v.Commission.CommissionRates.Rate, 64); err == nil {
					if rate > 1 {
						rate = rate / 1e18
					}
					commission = fmt.Sprintf("%.0f%%", rate*100)
				}
			}

			info := MyValidatorInfo{
				IsValidator:                  true,
				Address:                      v.OperatorAddress,
				Moniker:                      v.Description.Moniker,
				Status:                       status,
				VotingPower:                  votingPower,
				VotingPct:                    votingPct,
				Commission:                   commission,
				Jailed:                       v.Jailed,
				ValidatorExistsWithSameMoniker: monikerConflict != "",
				ConflictingMoniker:            monikerConflict,
			}

			// If jailed, fetch slashing info with timeout (10s)
			if v.Jailed {
				slashCtx, slashCancel := context.WithTimeout(context.Background(), 10*time.Second)
				slashingInfo, err := GetSlashingInfo(slashCtx, cfg, fullPubkeyJSON)
				slashCancel()
				if err == nil {
					info.SlashingInfo = slashingInfo
				} else {
					// Store error for display in UI
					info.SlashingInfoError = fmt.Sprintf("Failed to fetch jail reason: %v", err)
				}
			}

			return info, nil
		}
	}

	// Not matched by consensus pubkey, check for keyring address match
	// (validator may have been created with a key in the local keyring)
	keyringAddrs := getKeyringAddresses(bin, cfg)
	for _, keyAddr := range keyringAddrs {
		for _, v := range result.Validators {
			// Check if validator's operator address matches a key in the keyring
			// Compare bech32 data without the 6-char checksum (differs between prefixes)
			cosmosPrefix := "push1"
			validatorPrefix := "pushvaloper1"
			const bech32ChecksumLen = 6

			keyAddrData := strings.TrimPrefix(keyAddr, cosmosPrefix)
			valAddrData := strings.TrimPrefix(v.OperatorAddress, validatorPrefix)

			if len(keyAddrData) > bech32ChecksumLen && len(valAddrData) > bech32ChecksumLen &&
				keyAddrData[:len(keyAddrData)-bech32ChecksumLen] == valAddrData[:len(valAddrData)-bech32ChecksumLen] {
				// Found validator controlled by a key in our keyring
				status := parseStatus(v.Status)

				var votingPower int64
				if v.Tokens != "" {
					if tokens, err := strconv.ParseFloat(v.Tokens, 64); err == nil {
						votingPower = int64(tokens / 1e18)
					}
				}

				var votingPct float64
				if totalVotingPower > 0 {
					votingPct = float64(votingPower) / float64(totalVotingPower)
				}

				commission := "0%"
				if v.Commission.CommissionRates.Rate != "" {
					if rate, err := strconv.ParseFloat(v.Commission.CommissionRates.Rate, 64); err == nil {
						if rate > 1 {
							rate = rate / 1e18
						}
						commission = fmt.Sprintf("%.0f%%", rate*100)
					}
				}

				// Return validator info but with IsValidator=false (no consensus pubkey match)
				// This indicates keyring match but consensus key mismatch
				return MyValidatorInfo{
					IsValidator:                    false,
					Address:                        v.OperatorAddress,
					Moniker:                        v.Description.Moniker,
					Status:                         status,
					VotingPower:                    votingPower,
					VotingPct:                      votingPct,
					Commission:                     commission,
					Jailed:                         v.Jailed,
					ValidatorExistsWithSameMoniker: false,
					ConflictingMoniker:            "",
				}, nil
			}
		}
	}

	// Not matched by consensus pubkey, check for moniker-based match
	// (validator may have been created with different key/node)
	if localMoniker != "" {
		for _, v := range result.Validators {
			if v.Description.Moniker == localMoniker {
				// Found validator by moniker but consensus pubkey doesn't match
				status := parseStatus(v.Status)

				var votingPower int64
				if v.Tokens != "" {
					if tokens, err := strconv.ParseFloat(v.Tokens, 64); err == nil {
						votingPower = int64(tokens / 1e18)
					}
				}

				var votingPct float64
				if totalVotingPower > 0 {
					votingPct = float64(votingPower) / float64(totalVotingPower)
				}

				commission := "0%"
				if v.Commission.CommissionRates.Rate != "" {
					if rate, err := strconv.ParseFloat(v.Commission.CommissionRates.Rate, 64); err == nil {
						if rate > 1 {
							rate = rate / 1e18
						}
						commission = fmt.Sprintf("%.0f%%", rate*100)
					}
				}

				// Return validator info but with IsValidator=false (no consensus pubkey match)
				// This indicates moniker match but key/node mismatch
				return MyValidatorInfo{
					IsValidator:                    false,
					Address:                        v.OperatorAddress,
					Moniker:                        v.Description.Moniker,
					Status:                         status,
					VotingPower:                    votingPower,
					VotingPct:                      votingPct,
					Commission:                     commission,
					Jailed:                         v.Jailed,
					ValidatorExistsWithSameMoniker: false,
					ConflictingMoniker:            "",
				}, nil
			}
		}
	}

	// Not registered as validator, but check for moniker conflicts
	return MyValidatorInfo{
		IsValidator:                  false,
		ValidatorExistsWithSameMoniker: monikerConflict != "",
		ConflictingMoniker:            monikerConflict,
	}, nil
}

// parseStatus converts bond status to human-readable format
func parseStatus(status string) string {
	switch status {
	case "BOND_STATUS_BONDED":
		return "BONDED"
	case "BOND_STATUS_UNBONDING":
		return "UNBONDING"
	case "BOND_STATUS_UNBONDED":
		return "UNBONDED"
	default:
		return status
	}
}

// parseProposalStatus converts proposal status to human-readable format
func parseProposalStatus(status string) string {
	switch status {
	case "PROPOSAL_STATUS_DEPOSIT_PERIOD":
		return "DEPOSIT"
	case "PROPOSAL_STATUS_VOTING_PERIOD":
		return "VOTING"
	case "PROPOSAL_STATUS_PASSED":
		return "PASSED"
	case "PROPOSAL_STATUS_REJECTED":
		return "REJECTED"
	case "PROPOSAL_STATUS_FAILED":
		return "FAILED"
	default:
		return status
	}
}

// GetProposals fetches all governance proposals with 30s caching
func (f *Fetcher) GetProposals(ctx context.Context, cfg config.Config) (ProposalList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Force fetch on first call (cache is zero-initialized)
	if f.proposalsTime.IsZero() {
		list, err := f.fetchProposals(ctx, cfg)
		if err != nil {
			return ProposalList{}, err
		}
		f.proposals = list
		f.proposalsTime = time.Now()
		return list, nil
	}

	// Return cached if still valid
	if time.Since(f.proposalsTime) < f.cacheTTL && f.proposals.Total > 0 {
		return f.proposals, nil
	}

	// Fetch fresh data
	list, err := f.fetchProposals(ctx, cfg)
	if err != nil {
		// Return stale cache if available
		if f.proposals.Total > 0 {
			return f.proposals, nil
		}
		return ProposalList{}, err
	}

	// Update cache
	f.proposals = list
	f.proposalsTime = time.Now()
	return list, nil
}

// fetchProposals queries all governance proposals from the network
func (f *Fetcher) fetchProposals(ctx context.Context, cfg config.Config) (ProposalList, error) {
	bin, err := resolvePchaindBin(cfg.HomeDir)
	if err != nil {
		return ProposalList{}, fmt.Errorf("pchaind not found: %w", err)
	}

	remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)
	cmd := commandContext(ctx, bin, "query", "gov", "proposals", "--node", remote, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return ProposalList{}, fmt.Errorf("query proposals failed: %w", err)
	}

	var result struct {
		Proposals []struct {
			ID       string `json:"id"`
			Messages []struct {
				Type    string `json:"@type"`
				Content struct {
					Title       string `json:"title"`
					Description string `json:"description"`
				} `json:"content,omitempty"`
				// For v1 gov proposals
				Title       string `json:"title,omitempty"`
				Description string `json:"description,omitempty"`
			} `json:"messages"`
			Status        string `json:"status"`
			VotingEndTime string `json:"voting_end_time"`
			// Legacy fields for older proposal formats
			Content struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			} `json:"content,omitempty"`
			Title string `json:"title,omitempty"`
		} `json:"proposals"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return ProposalList{}, fmt.Errorf("parse proposals failed: %w", err)
	}

	proposals := make([]Proposal, 0, len(result.Proposals))
	for _, p := range result.Proposals {
		// Extract title from various possible locations
		title := p.Title
		if title == "" && p.Content.Title != "" {
			title = p.Content.Title
		}
		if title == "" && len(p.Messages) > 0 {
			if p.Messages[0].Title != "" {
				title = p.Messages[0].Title
			} else if p.Messages[0].Content.Title != "" {
				title = p.Messages[0].Content.Title
			}
		}
		if title == "" {
			title = "Untitled Proposal"
		}

		// Extract description
		description := ""
		if p.Content.Description != "" {
			description = p.Content.Description
		} else if len(p.Messages) > 0 {
			if p.Messages[0].Description != "" {
				description = p.Messages[0].Description
			} else if p.Messages[0].Content.Description != "" {
				description = p.Messages[0].Content.Description
			}
		}

		status := parseProposalStatus(p.Status)

		// Include voting end time if available (not default epoch time)
		votingEnd := ""
		if p.VotingEndTime != "" && p.VotingEndTime != "0001-01-01T00:00:00Z" {
			votingEnd = p.VotingEndTime
		}

		proposals = append(proposals, Proposal{
			ID:          p.ID,
			Title:       title,
			Status:      status,
			VotingEnd:   votingEnd,
			Description: description,
		})
	}

	return ProposalList{
		Proposals: proposals,
		Total:     len(proposals),
	}, nil
}

// GetCachedProposals returns cached proposals list
func GetCachedProposals(ctx context.Context, cfg config.Config) (ProposalList, error) {
	return globalFetcher.GetProposals(ctx, cfg)
}

// GetValidatorRewards fetches commission and outstanding rewards for a validator
// Both queries are executed in parallel for better performance.
func GetValidatorRewards(ctx context.Context, cfg config.Config, validatorAddr string) (commission string, outstanding string, err error) {
	if validatorAddr == "" {
		return "—", "—", fmt.Errorf("validator address required")
	}

	bin, err := resolvePchaindBin(cfg.HomeDir)
	if err != nil {
		return "—", "—", fmt.Errorf("pchaind not found: %w", err)
	}

	// Use the provided context or create one with timeout
	queryCtx := ctx
	if queryCtx == nil {
		var cancel context.CancelFunc
		queryCtx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
	}

	remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)

	// Fetch commission and outstanding rewards in parallel
	var wg sync.WaitGroup
	commissionRewards := "—"
	outstandingRewards := "—"

	wg.Add(2)

	// Fetch commission rewards
	go func() {
		defer wg.Done()
		commCmd := commandContext(queryCtx, bin, "query", "distribution", "commission", validatorAddr, "--node", remote, "-o", "json")
		if commOutput, err := commCmd.Output(); err == nil {
			var commResult struct {
				Commission struct {
					Commission []string `json:"commission"`
				} `json:"commission"`
			}
			if err := json.Unmarshal(commOutput, &commResult); err == nil && len(commResult.Commission.Commission) > 0 {
				amountStr := commResult.Commission.Commission[0]
				amountStr = strings.TrimSuffix(amountStr, "upc")
				if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
					commissionRewards = fmt.Sprintf("%.2f", amount/1e18)
				}
			}
		}
	}()

	// Fetch outstanding rewards (no retry for speed)
	go func() {
		defer wg.Done()
		outCmd := commandContext(queryCtx, bin, "query", "distribution", "validator-outstanding-rewards", validatorAddr, "--node", remote, "-o", "json")
		if outOutput, err := outCmd.Output(); err == nil {
			var outResult struct {
				Rewards struct {
					Rewards []string `json:"rewards"`
				} `json:"rewards"`
			}
			if err := json.Unmarshal(outOutput, &outResult); err == nil && len(outResult.Rewards.Rewards) > 0 {
				amountStr := outResult.Rewards.Rewards[0]
				amountStr = strings.TrimSuffix(amountStr, "upc")
				if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
					outstandingRewards = fmt.Sprintf("%.2f", amount/1e18)
				}
			}
		}
	}()

	wg.Wait()
	return commissionRewards, outstandingRewards, nil
}

// GetCachedValidatorRewards fetches validator rewards with 30s caching
func (f *Fetcher) GetCachedValidatorRewards(ctx context.Context, cfg config.Config, validatorAddr string) (commission string, outstanding string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check cache first
	if cached, exists := f.rewardsCache[validatorAddr]; exists {
		if time.Since(cached.fetchedAt) < f.rewardsTTL {
			return cached.commission, cached.outstanding, nil
		}
	}

	// Cache miss or expired - fetch fresh data
	commission, outstanding, err = GetValidatorRewards(ctx, cfg, validatorAddr)
	if err == nil {
		// Store in cache
		f.rewardsCache[validatorAddr] = rewardsCacheEntry{
			commission:  commission,
			outstanding: outstanding,
			fetchedAt:   time.Now(),
		}
	}

	return commission, outstanding, err
}

// Global fetcher instance
var globalFetcher = NewFetcher()

// GetCachedValidatorsList returns cached validator list
func GetCachedValidatorsList(ctx context.Context, cfg config.Config) (ValidatorList, error) {
	return globalFetcher.GetAllValidators(ctx, cfg)
}

// GetCachedMyValidator returns cached my validator info
func GetCachedMyValidator(ctx context.Context, cfg config.Config) (MyValidatorInfo, error) {
	return globalFetcher.GetMyValidator(ctx, cfg)
}

// GetCachedRewards returns validator rewards with 30s caching
func GetCachedRewards(ctx context.Context, cfg config.Config, validatorAddr string) (commission string, outstanding string, err error) {
	return globalFetcher.GetCachedValidatorRewards(ctx, cfg, validatorAddr)
}

// GetEVMAddress converts a Cosmos validator address to EVM address
func GetEVMAddress(ctx context.Context, validatorAddr string) string {
	if validatorAddr == "" {
		return "—"
	}

	homeDir := config.Defaults().HomeDir
	bin, err := resolvePchaindBin(homeDir)
	if err != nil {
		return "—"
	}

	cmd := commandContext(ctx, bin, "debug", "addr", validatorAddr)
	output, err := cmd.Output()
	if err != nil {
		return "—"
	}

	// Parse output to extract hex address
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Address (hex):") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				hex := strings.TrimSpace(parts[1])
				return "0x" + hex
			}
		}
	}

	return "—"
}

// GetSlashingInfo fetches slashing information for a validator (jail reason, jailed until time, etc)
func GetSlashingInfo(ctx context.Context, cfg config.Config, consensusPubkey string) (SlashingInfo, error) {
	bin, err := resolvePchaindBin(cfg.HomeDir)
	if err != nil {
		return SlashingInfo{}, fmt.Errorf("pchaind not found: %w", err)
	}

	remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)

	// Query signing info to get jail details
	// consensusPubkey should be a JSON string like: {"@type":"/cosmos.crypto.ed25519.PubKey","key":"..."}
	cmd := commandContext(ctx, bin, "query", "slashing", "signing-info", consensusPubkey, "--node", remote, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return SlashingInfo{}, fmt.Errorf("failed to query slashing info: %w", err)
	}

	var result struct {
		ValSigningInfo struct {
			Address      string `json:"address"`
			StartHeight  string `json:"start_height"`
			JailedUntil  string `json:"jailed_until"`
			Tombstoned   bool   `json:"tombstoned"`
			MissedBlocks string `json:"missed_blocks_counter"`
		} `json:"val_signing_info"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return SlashingInfo{}, fmt.Errorf("failed to parse slashing info: %w", err)
	}

	info := SlashingInfo{
		Tombstoned:  result.ValSigningInfo.Tombstoned,
		JailedUntil: result.ValSigningInfo.JailedUntil,
	}

	// Parse missed blocks counter
	if result.ValSigningInfo.MissedBlocks != "" {
		if mb, err := strconv.ParseInt(result.ValSigningInfo.MissedBlocks, 10, 64); err == nil {
			info.MissedBlocks = mb
		}
	}

	// Determine jail reason with better heuristics
	if info.Tombstoned {
		info.JailReason = "Double Sign"
	} else if info.JailedUntil != "" && info.JailedUntil != "1970-01-01T00:00:00Z" {
		// Valid jail time (not epoch) indicates downtime
		info.JailReason = "Downtime"
	} else if info.MissedBlocks > 0 {
		// If missed blocks > 0, it's likely a downtime issue
		info.JailReason = "Downtime"
	} else {
		// Unable to determine reason
		info.JailReason = "Unknown"
	}

	return info, nil
}

// getKeyringAddresses returns all addresses in the local keyring
func getKeyringAddresses(bin string, cfg config.Config) []string {
	var addresses []string

	// List all keys in the keyring
	cmd := commandContext(context.Background(), bin, "keys", "list", "--keyring-backend", cfg.KeyringBackend, "--home", cfg.HomeDir, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return addresses
	}

	// Parse the JSON output to extract addresses
	var keys []struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(output, &keys); err != nil {
		return addresses
	}

	for _, key := range keys {
		if key.Address != "" {
			addresses = append(addresses, key.Address)
		}
	}

	return addresses
}
