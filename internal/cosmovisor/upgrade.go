package cosmovisor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// UpgradeInfo represents the upgrade-info.json structure for Cosmovisor.
type UpgradeInfo struct {
	Name   string `json:"name"`
	Info   string `json:"info"`
	Height int64  `json:"height,omitempty"`
}

// BinaryInfo represents the binaries section in upgrade info.
type BinaryInfo struct {
	Binaries map[string]string `json:"binaries"`
}

// Platform represents an OS/architecture combination.
type Platform struct {
	OS   string
	Arch string
}

// DefaultPlatforms returns the standard platforms to build upgrade info for.
func DefaultPlatforms() []Platform {
	return []Platform{
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "arm64"},
	}
}

// GenerateUpgradeInfoOptions contains options for generating upgrade info.
type GenerateUpgradeInfoOptions struct {
	Version     string     // Version name (e.g., "v1.1.0")
	BaseURL     string     // Base URL for binary downloads
	Height      int64      // Optional: upgrade height
	Platforms   []Platform // Optional: platforms to include (defaults to DefaultPlatforms)
	ProjectName string     // Optional: project name for archive filename (default: "push-chain")
	Progress    func(msg string)
}

// GenerateUpgradeInfo creates upgrade JSON with checksums for all specified platforms.
func GenerateUpgradeInfo(ctx context.Context, opts GenerateUpgradeInfoOptions) (*UpgradeInfo, error) {
	if opts.Version == "" {
		return nil, fmt.Errorf("version is required")
	}
	if opts.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	platforms := opts.Platforms
	if len(platforms) == 0 {
		platforms = DefaultPlatforms()
	}

	projectName := opts.ProjectName
	if projectName == "" {
		projectName = "push-chain"
	}

	progress := opts.Progress
	if progress == nil {
		progress = func(string) {}
	}

	// Strip 'v' prefix for GoReleaser archive naming
	versionNum := strings.TrimPrefix(opts.Version, "v")

	binaries := make(map[string]string)
	baseURL := strings.TrimRight(opts.BaseURL, "/")

	for _, p := range platforms {
		// Construct archive filename (GoReleaser format)
		// Example: push-chain_1.0.0_linux_amd64.tar.gz
		archiveName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", projectName, versionNum, p.OS, p.Arch)
		archiveURL := fmt.Sprintf("%s/%s", baseURL, archiveName)

		progress(fmt.Sprintf("Fetching %s/%s binary...", p.OS, p.Arch))

		// Calculate checksum
		checksum, err := fetchAndHash(ctx, archiveURL)
		if err != nil {
			progress(fmt.Sprintf("Warning: failed to fetch %s/%s: %v", p.OS, p.Arch, err))
			continue
		}

		key := fmt.Sprintf("%s/%s", p.OS, p.Arch)
		binaries[key] = fmt.Sprintf("%s?checksum=sha256:%s", archiveURL, checksum)
		progress(fmt.Sprintf("Got checksum for %s/%s: %s", p.OS, p.Arch, checksum[:16]+"..."))
	}

	if len(binaries) == 0 {
		return nil, fmt.Errorf("no binaries found at %s", opts.BaseURL)
	}

	// Create info JSON string
	infoBytes, err := json.Marshal(BinaryInfo{Binaries: binaries})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal binary info: %w", err)
	}

	return &UpgradeInfo{
		Name:   opts.Version,
		Info:   string(infoBytes),
		Height: opts.Height,
	}, nil
}

// fetchAndHash downloads a file and returns its SHA256 hash.
func fetchAndHash(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, resp.Body); err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// FormatUpgradeProposal returns a formatted upgrade proposal JSON for governance.
func FormatUpgradeProposal(info *UpgradeInfo, authority string, deposit string, title string, summary string) ([]byte, error) {
	proposal := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"@type":     "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
				"authority": authority,
				"plan": map[string]interface{}{
					"name":   info.Name,
					"height": fmt.Sprintf("%d", info.Height),
					"info":   info.Info,
				},
			},
		},
		"deposit": deposit,
		"title":   title,
		"summary": summary,
	}

	return json.MarshalIndent(proposal, "", "  ")
}
