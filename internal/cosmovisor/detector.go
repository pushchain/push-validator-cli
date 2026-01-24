package cosmovisor

import (
	"os"
	"os/exec"
	"path/filepath"
)

// DetectionResult contains the result of Cosmovisor detection.
type DetectionResult struct {
	Available     bool   // cosmovisor binary found in PATH or env
	BinaryPath    string // path to cosmovisor binary
	SetupComplete bool   // ~/.pchain/cosmovisor/genesis/bin/pchaind exists
	ShouldUse     bool   // Available (setup can be auto-initialized)
	Reason        string // Human-readable reason
}

// Detect checks if Cosmovisor is available and properly set up.
func Detect(homeDir string) DetectionResult {
	result := DetectionResult{}

	// Check for cosmovisor binary
	cosmovisorPath := findCosmovisor()
	if cosmovisorPath != "" {
		result.Available = true
		result.BinaryPath = cosmovisorPath
	}

	// Check for setup completion
	genesisPath := filepath.Join(homeDir, "cosmovisor", "genesis", "bin", "pchaind")
	if _, err := os.Stat(genesisPath); err == nil {
		result.SetupComplete = true
	}

	// Determine if Cosmovisor should be used
	// We use Cosmovisor if the binary is available (setup can be auto-initialized)
	result.ShouldUse = result.Available

	// Set reason
	switch {
	case result.Available && result.SetupComplete:
		result.Reason = "Cosmovisor is available and properly configured"
	case result.Available && !result.SetupComplete:
		result.Reason = "Cosmovisor is available (will auto-initialize on start)"
	default:
		result.Reason = "cosmovisor binary not found in PATH"
	}

	return result
}

// findCosmovisor returns the path to cosmovisor or empty string if not found.
func findCosmovisor() string {
	// Check COSMOVISOR env first
	if v := os.Getenv("COSMOVISOR"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}

	// Check PATH lookup
	if path, err := exec.LookPath("cosmovisor"); err == nil {
		return path
	}

	// Check common Go binary locations (go install puts binaries here)
	candidates := []string{}
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		candidates = append(candidates, filepath.Join(gobin, "cosmovisor"))
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		candidates = append(candidates, filepath.Join(gopath, "bin", "cosmovisor"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "go", "bin", "cosmovisor"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	return ""
}

// IsAvailable returns true if cosmovisor binary is found.
func IsAvailable() bool {
	return findCosmovisor() != ""
}

// BinaryPath returns the path to the cosmovisor binary, or empty string if not found.
func BinaryPath() string {
	return findCosmovisor()
}
