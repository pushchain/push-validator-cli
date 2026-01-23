package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/cosmovisor"
)

func TestCosmovisorStatusCore_JSON_NotAvailable(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	detection := cosmovisor.DetectionResult{
		Available:     false,
		SetupComplete: false,
		ShouldUse:     false,
		Reason:        "cosmovisor binary not found",
	}

	err := cosmovisorStatusCore(detection, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCosmovisorStatusCore_JSON_Available(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	detection := cosmovisor.DetectionResult{
		Available:     true,
		BinaryPath:    "/usr/local/bin/cosmovisor",
		SetupComplete: true,
		ShouldUse:     true,
		Reason:        "cosmovisor ready",
	}

	status := &cosmovisor.Status{
		Installed:       true,
		GenesisVersion:  "v1.0.0",
		CurrentVersion:  "v1.1.0",
		ActiveBinary:    "/home/user/.pchain/cosmovisor/current/bin/pchaind",
		PendingUpgrades: []string{"v1.2.0"},
	}

	err := cosmovisorStatusCore(detection, status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCosmovisorStatusCore_Text_NotAvailable(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	detection := cosmovisor.DetectionResult{
		Available:     false,
		SetupComplete: false,
		ShouldUse:     false,
	}

	err := cosmovisorStatusCore(detection, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCosmovisorStatusCore_Text_Available_WithStatus(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	detection := cosmovisor.DetectionResult{
		Available:     true,
		BinaryPath:    "/usr/local/bin/cosmovisor",
		SetupComplete: true,
		ShouldUse:     true,
	}

	status := &cosmovisor.Status{
		Installed:       true,
		GenesisVersion:  "v1.0.0",
		CurrentVersion:  "v1.1.0",
		ActiveBinary:    "/home/user/.pchain/cosmovisor/current/bin/pchaind",
		PendingUpgrades: []string{"v1.2.0", "v1.3.0"},
	}

	err := cosmovisorStatusCore(detection, status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCosmovisorStatusCore_Text_Available_NilStatus(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	detection := cosmovisor.DetectionResult{
		Available:     true,
		BinaryPath:    "/usr/local/bin/cosmovisor",
		SetupComplete: false,
		ShouldUse:     true,
	}

	err := cosmovisorStatusCore(detection, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCosmovisorUpgradeInfoCore_MissingVersion(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	err := cosmovisorUpgradeInfoCore(context.Background(), "", "https://example.com", 0, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "--version is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCosmovisorUpgradeInfoCore_MissingURL(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	err := cosmovisorUpgradeInfoCore(context.Background(), "v1.1.0", "", 0, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "--url is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCosmovisorUpgradeInfoCore_GeneratorError_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	generator := func(ctx context.Context, opts cosmovisor.GenerateUpgradeInfoOptions) (*cosmovisor.UpgradeInfo, error) {
		return nil, fmt.Errorf("download failed: 404")
	}

	err := cosmovisorUpgradeInfoCore(context.Background(), "v1.1.0", "https://example.com/releases", 0, generator)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "download failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCosmovisorUpgradeInfoCore_GeneratorError_Text(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true

	generator := func(ctx context.Context, opts cosmovisor.GenerateUpgradeInfoOptions) (*cosmovisor.UpgradeInfo, error) {
		return nil, fmt.Errorf("network timeout")
	}

	err := cosmovisorUpgradeInfoCore(context.Background(), "v1.1.0", "https://example.com/releases", 10000, generator)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCosmovisorUpgradeInfoCore_Success_JSON(t *testing.T) {
	origOutput := flagOutput
	defer func() { flagOutput = origOutput }()
	flagOutput = "json"

	generator := func(ctx context.Context, opts cosmovisor.GenerateUpgradeInfoOptions) (*cosmovisor.UpgradeInfo, error) {
		if opts.Version != "v1.1.0" {
			t.Errorf("expected version v1.1.0, got %s", opts.Version)
		}
		if opts.BaseURL != "https://example.com/releases" {
			t.Errorf("expected URL, got %s", opts.BaseURL)
		}
		if opts.Height != 50000 {
			t.Errorf("expected height 50000, got %d", opts.Height)
		}
		return &cosmovisor.UpgradeInfo{
			Name:   "v1.1.0",
			Info:   `{"binaries":{"linux/amd64":"https://example.com/releases/v1.1.0/pchaind-linux-amd64?checksum=sha256:abc123"}}`,
			Height: 50000,
		}, nil
	}

	err := cosmovisorUpgradeInfoCore(context.Background(), "v1.1.0", "https://example.com/releases", 50000, generator)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCosmovisorUpgradeInfoCore_Success_Text(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origQuiet := flagQuiet
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagQuiet = origQuiet
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true
	flagQuiet = false

	generator := func(ctx context.Context, opts cosmovisor.GenerateUpgradeInfoOptions) (*cosmovisor.UpgradeInfo, error) {
		// Call the progress function to test that code path
		if opts.Progress != nil {
			opts.Progress("Downloading linux/amd64...")
		}
		return &cosmovisor.UpgradeInfo{
			Name:   "v1.1.0",
			Info:   `{"binaries":{}}`,
			Height: 0,
		}, nil
	}

	err := cosmovisorUpgradeInfoCore(context.Background(), "v1.1.0", "https://example.com/releases", 0, generator)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCosmovisorUpgradeInfoCore_Success_Quiet(t *testing.T) {
	origOutput := flagOutput
	origNoColor := flagNoColor
	origNoEmoji := flagNoEmoji
	origQuiet := flagQuiet
	defer func() {
		flagOutput = origOutput
		flagNoColor = origNoColor
		flagNoEmoji = origNoEmoji
		flagQuiet = origQuiet
	}()
	flagOutput = "text"
	flagNoColor = true
	flagNoEmoji = true
	flagQuiet = true

	generator := func(ctx context.Context, opts cosmovisor.GenerateUpgradeInfoOptions) (*cosmovisor.UpgradeInfo, error) {
		// Progress should not print in quiet mode
		if opts.Progress != nil {
			opts.Progress("should not print")
		}
		return &cosmovisor.UpgradeInfo{
			Name: "v1.1.0",
			Info: `{}`,
		}, nil
	}

	err := cosmovisorUpgradeInfoCore(context.Background(), "v1.1.0", "https://example.com/releases", 0, generator)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
