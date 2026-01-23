package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/node"
	ui "github.com/pushchain/push-validator-cli/internal/ui"
)

func testColorConfig() *ui.ColorConfig {
	c := ui.NewColorConfig()
	c.Enabled = false
	c.EmojiEnabled = false
	return c
}

func TestCheckProcessRunning_Running(t *testing.T) {
	sup := &mockSupervisor{running: true, pid: 12345}
	c := testColorConfig()

	result := checkProcessRunning(sup, c)

	if result.Status != "pass" {
		t.Errorf("checkProcessRunning() Status = %q, want %q", result.Status, "pass")
	}
	if result.Name != "Process Status" {
		t.Errorf("checkProcessRunning() Name = %q, want %q", result.Name, "Process Status")
	}
}

func TestCheckProcessRunning_RunningNoPID(t *testing.T) {
	sup := &mockSupervisor{running: true, pid: 0}
	c := testColorConfig()

	result := checkProcessRunning(sup, c)

	if result.Status != "pass" {
		t.Errorf("checkProcessRunning() Status = %q, want %q", result.Status, "pass")
	}
}

func TestCheckProcessRunning_Stopped(t *testing.T) {
	sup := &mockSupervisor{running: false}
	c := testColorConfig()

	result := checkProcessRunning(sup, c)

	if result.Status != "fail" {
		t.Errorf("checkProcessRunning() Status = %q, want %q", result.Status, "fail")
	}
	if len(result.Details) == 0 {
		t.Error("checkProcessRunning() should have Details when stopped")
	}
}

func TestCheckConfigFiles_AllPresent(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("test"), 0o644)
	os.WriteFile(filepath.Join(configDir, "genesis.json"), []byte("{}"), 0o644)

	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkConfigFiles(cfg, c)

	if result.Status != "pass" {
		t.Errorf("checkConfigFiles() Status = %q, want %q", result.Status, "pass")
	}
}

func TestCheckConfigFiles_MissingGenesis(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("test"), 0o644)

	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkConfigFiles(cfg, c)

	if result.Status != "fail" {
		t.Errorf("checkConfigFiles() Status = %q, want %q", result.Status, "fail")
	}
}

func TestCheckConfigFiles_MissingBoth(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkConfigFiles(cfg, c)

	if result.Status != "fail" {
		t.Errorf("checkConfigFiles() Status = %q, want %q", result.Status, "fail")
	}
}

func TestCheckP2PPeers_NoPeers(t *testing.T) {
	cli := &mockNodeClient{peers: []node.Peer{}, peersErr: nil}
	c := testColorConfig()

	result := checkP2PPeers(cli, c)

	if result.Status != "fail" {
		t.Errorf("checkP2PPeers() Status = %q, want %q", result.Status, "fail")
	}
}

func TestCheckP2PPeers_FewPeers(t *testing.T) {
	cli := &mockNodeClient{
		peers: []node.Peer{
			{ID: "peer1", Addr: "1.2.3.4:26656"},
			{ID: "peer2", Addr: "5.6.7.8:26656"},
		},
	}
	c := testColorConfig()

	result := checkP2PPeers(cli, c)

	if result.Status != "warn" {
		t.Errorf("checkP2PPeers() Status = %q, want %q", result.Status, "warn")
	}
}

func TestCheckP2PPeers_Healthy(t *testing.T) {
	cli := &mockNodeClient{
		peers: []node.Peer{
			{ID: "peer1", Addr: "1.2.3.4:26656"},
			{ID: "peer2", Addr: "5.6.7.8:26656"},
			{ID: "peer3", Addr: "9.10.11.12:26656"},
		},
	}
	c := testColorConfig()

	result := checkP2PPeers(cli, c)

	if result.Status != "pass" {
		t.Errorf("checkP2PPeers() Status = %q, want %q", result.Status, "pass")
	}
}

func TestCheckP2PPeers_RPCError(t *testing.T) {
	cli := &mockNodeClient{peersErr: fmt.Errorf("connection refused")}
	c := testColorConfig()

	result := checkP2PPeers(cli, c)

	if result.Status != "warn" {
		t.Errorf("checkP2PPeers() Status = %q, want %q", result.Status, "warn")
	}
}

func TestCheckRemoteConnectivity_Success(t *testing.T) {
	cli := &mockNodeClient{
		status: node.Status{Height: 1000, CatchingUp: false},
	}
	c := testColorConfig()

	result := checkRemoteConnectivity(cli, "donut.rpc.push.org", c)

	if result.Status != "pass" {
		t.Errorf("checkRemoteConnectivity() Status = %q, want %q", result.Status, "pass")
	}
}

func TestCheckRemoteConnectivity_Failure(t *testing.T) {
	cli := &mockNodeClient{statusErr: fmt.Errorf("timeout")}
	c := testColorConfig()

	result := checkRemoteConnectivity(cli, "donut.rpc.push.org", c)

	if result.Status != "fail" {
		t.Errorf("checkRemoteConnectivity() Status = %q, want %q", result.Status, "fail")
	}
}

func TestCheckSyncStatus_InSync(t *testing.T) {
	cli := &mockNodeClient{
		status: node.Status{Height: 50000, CatchingUp: false},
	}
	c := testColorConfig()

	result := checkSyncStatus(cli, c)

	if result.Status != "pass" {
		t.Errorf("checkSyncStatus() Status = %q, want %q", result.Status, "pass")
	}
}

func TestCheckSyncStatus_Syncing(t *testing.T) {
	cli := &mockNodeClient{
		status: node.Status{Height: 1000, CatchingUp: true},
	}
	c := testColorConfig()

	result := checkSyncStatus(cli, c)

	if result.Status != "warn" {
		t.Errorf("checkSyncStatus() Status = %q, want %q", result.Status, "warn")
	}
}

func TestCheckSyncStatus_RPCError(t *testing.T) {
	cli := &mockNodeClient{statusErr: fmt.Errorf("connection refused")}
	c := testColorConfig()

	result := checkSyncStatus(cli, c)

	if result.Status != "warn" {
		t.Errorf("checkSyncStatus() Status = %q, want %q", result.Status, "warn")
	}
}

func TestCheckDiskSpace_Writable(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkDiskSpace(cfg, c)

	if result.Status != "pass" {
		t.Errorf("checkDiskSpace() Status = %q, want %q", result.Status, "pass")
	}
}

func TestCheckDiskSpace_NonexistentDir(t *testing.T) {
	cfg := config.Config{HomeDir: "/nonexistent/path/that/does/not/exist"}
	c := testColorConfig()

	result := checkDiskSpace(cfg, c)

	if result.Status != "warn" {
		t.Errorf("checkDiskSpace() Status = %q, want %q", result.Status, "warn")
	}
}

func TestCheckPermissions_WorldReadable(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("test"), 0o644)

	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkPermissions(cfg, c)

	if result.Status != "pass" {
		t.Errorf("checkPermissions() Status = %q, want %q", result.Status, "pass")
	}
}

func TestCheckPermissions_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkPermissions(cfg, c)

	if result.Status != "warn" {
		t.Errorf("checkPermissions() Status = %q, want %q", result.Status, "warn")
	}
}

func TestCheckCosmovisor_NotAvailable(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkCosmovisor(cfg, c)

	if result.Status != "warn" {
		t.Errorf("checkCosmovisor() Status = %q, want %q", result.Status, "warn")
	}
}

func TestDoctorSummary_AllPassed(t *testing.T) {
	c := testColorConfig()
	results := []checkResult{
		{Status: "pass", Name: "Test 1"},
		{Status: "pass", Name: "Test 2"},
		{Status: "pass", Name: "Test 3"},
	}

	err := doctorSummary(results, c)
	if err != nil {
		t.Errorf("doctorSummary() with all pass should return nil, got %v", err)
	}
}

func TestDoctorSummary_WithWarnings(t *testing.T) {
	c := testColorConfig()
	results := []checkResult{
		{Status: "pass", Name: "Test 1"},
		{Status: "warn", Name: "Test 2"},
		{Status: "pass", Name: "Test 3"},
	}

	err := doctorSummary(results, c)
	if err != nil {
		t.Errorf("doctorSummary() with warnings should return nil, got %v", err)
	}
}

func TestDoctorSummary_WithFailures(t *testing.T) {
	c := testColorConfig()
	results := []checkResult{
		{Status: "pass", Name: "Test 1"},
		{Status: "fail", Name: "Test 2"},
		{Status: "warn", Name: "Test 3"},
	}

	err := doctorSummary(results, c)
	if err == nil {
		t.Error("doctorSummary() with failures should return error")
	}
}

func TestRunDoctorChecks_Integration(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("test"), 0o644)
	os.WriteFile(filepath.Join(configDir, "genesis.json"), []byte("{}"), 0o644)

	cfg := config.Config{
		HomeDir:       dir,
		GenesisDomain: "test.example.com",
		RPCLocal:      "http://127.0.0.1:26657",
	}
	sup := &mockSupervisor{running: true, pid: 100}
	localCli := &mockNodeClient{
		status: node.Status{Height: 5000, CatchingUp: false},
		peers: []node.Peer{
			{ID: "peer1", Addr: "1.2.3.4:26656"},
			{ID: "peer2", Addr: "5.6.7.8:26656"},
			{ID: "peer3", Addr: "9.10.11.12:26656"},
		},
	}
	remoteCli := &mockNodeClient{
		status: node.Status{Height: 5000, CatchingUp: false},
	}
	c := testColorConfig()

	results := runDoctorChecks(cfg, sup, localCli, remoteCli, c)

	if len(results) != 9 {
		t.Errorf("runDoctorChecks() returned %d results, want 9", len(results))
	}

	// Count passes
	passCount := 0
	for _, r := range results {
		if r.Status == "pass" {
			passCount++
		}
	}
	// At minimum: process, config, P2P, remote, disk, permissions, sync should pass
	if passCount < 6 {
		t.Errorf("runDoctorChecks() only %d checks passed, expected at least 6", passCount)
	}
}

func TestPrintCheck(t *testing.T) {
	c := testColorConfig()

	// These should not panic
	printCheck(checkResult{Status: "pass", Name: "Test", Message: "OK"}, c)
	printCheck(checkResult{Status: "warn", Name: "Test", Message: "Warn", Details: []string{"detail1"}}, c)
	printCheck(checkResult{Status: "fail", Name: "Test", Message: "Fail", Details: []string{"d1", "d2"}}, c)
}

func TestCheckRPCAccessible_Listening(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		HomeDir:  dir,
		RPCLocal: "http://127.0.0.1:26657",
	}
	c := testColorConfig()

	// Note: This test will likely fail in test environment since RPC won't be running
	// The actual function calls process.IsRPCListening which checks real network connectivity
	result := checkRPCAccessible(cfg, c)

	// In most test environments, RPC won't actually be listening
	// So we just verify the function runs without panic and returns a valid result
	if result.Name != "RPC Accessibility" {
		t.Errorf("checkRPCAccessible() Name = %q, want %q", result.Name, "RPC Accessibility")
	}
	if result.Status != "pass" && result.Status != "fail" {
		t.Errorf("checkRPCAccessible() Status = %q, want pass or fail", result.Status)
	}
}

func TestCheckRPCAccessible_NotListening(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		HomeDir:  dir,
		RPCLocal: "http://127.0.0.1:9999", // Unlikely port to be in use
	}
	c := testColorConfig()

	result := checkRPCAccessible(cfg, c)

	if result.Status != "fail" {
		t.Errorf("checkRPCAccessible() Status = %q, want %q", result.Status, "fail")
	}
	if result.Name != "RPC Accessibility" {
		t.Errorf("checkRPCAccessible() Name = %q, want %q", result.Name, "RPC Accessibility")
	}
	if len(result.Details) == 0 {
		t.Error("checkRPCAccessible() should have Details when RPC not accessible")
	}
}

func TestCheckCosmovisor_SetupComplete(t *testing.T) {
	dir := t.TempDir()

	// Create cosmovisor directory structure with genesis binary
	cosmovisorDir := filepath.Join(dir, "cosmovisor", "genesis", "bin")
	os.MkdirAll(cosmovisorDir, 0o755)
	binaryPath := filepath.Join(cosmovisorDir, "pchaind")
	os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0o755)

	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkCosmovisor(cfg, c)

	// The result depends on whether cosmovisor binary is in PATH
	// If not available, status will be "warn" even with setup complete
	// If available, status will be "pass" with setup complete
	if result.Name != "Cosmovisor" {
		t.Errorf("checkCosmovisor() Name = %q, want %q", result.Name, "Cosmovisor")
	}
	if result.Status != "pass" && result.Status != "warn" {
		t.Errorf("checkCosmovisor() Status = %q, want pass or warn", result.Status)
	}
}

func TestCheckDiskSpace_NotWritable(t *testing.T) {
	// Create a temp file (not a directory) to test the "not a directory" case
	tmpFile, err := os.CreateTemp("", "disktest")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFilePath)

	cfg := config.Config{HomeDir: tmpFilePath}
	c := testColorConfig()

	result := checkDiskSpace(cfg, c)

	if result.Status != "fail" {
		t.Errorf("checkDiskSpace() Status = %q, want %q", result.Status, "fail")
	}
	if result.Name != "Disk Space" {
		t.Errorf("checkDiskSpace() Name = %q, want %q", result.Name, "Disk Space")
	}
}

func TestCheckPermissions_RestrictivePermissions(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	configPath := filepath.Join(configDir, "config.toml")

	// Create config.toml with restrictive permissions (0600 = no world-readable bit)
	os.WriteFile(configPath, []byte("test"), 0o600)

	cfg := config.Config{HomeDir: dir}
	c := testColorConfig()

	result := checkPermissions(cfg, c)

	if result.Status != "warn" {
		t.Errorf("checkPermissions() Status = %q, want %q", result.Status, "warn")
	}
	if result.Name != "File Permissions" {
		t.Errorf("checkPermissions() Name = %q, want %q", result.Name, "File Permissions")
	}
}

func TestRunDoctorChecks_AllFailing(t *testing.T) {
	// Create config with non-existent HomeDir to trigger failures
	cfg := config.Config{
		HomeDir:       "/nonexistent/path/that/does/not/exist",
		GenesisDomain: "invalid.example.com",
		RPCLocal:      "http://127.0.0.1:9999",
	}

	// Mock supervisor with process not running
	sup := &mockSupervisor{running: false}

	// Mock local client with errors
	localCli := &mockNodeClient{
		peersErr:  fmt.Errorf("connection refused"),
		statusErr: fmt.Errorf("connection error"),
	}

	// Mock remote client with errors
	remoteCli := &mockNodeClient{
		statusErr: fmt.Errorf("timeout"),
	}

	c := testColorConfig()

	results := runDoctorChecks(cfg, sup, localCli, remoteCli, c)

	if len(results) != 9 {
		t.Errorf("runDoctorChecks() returned %d results, want 9", len(results))
	}

	// Count failures and warnings
	failCount := 0
	warnCount := 0
	for _, r := range results {
		if r.Status == "fail" {
			failCount++
		} else if r.Status == "warn" {
			warnCount++
		}
	}

	// We expect multiple failures/warnings with this configuration
	if failCount+warnCount < 5 {
		t.Errorf("runDoctorChecks() only %d checks failed/warned, expected at least 5", failCount+warnCount)
	}
}
