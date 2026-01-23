package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/pushchain/push-validator-cli/internal/exitcodes"
	syncmon "github.com/pushchain/push-validator-cli/internal/sync"
)

// mockSyncRunner implements SyncRunner for testing.
type mockSyncRunner struct {
	err  error
	opts syncmon.Options
}

func (m *mockSyncRunner) Run(_ context.Context, opts syncmon.Options) error {
	m.opts = opts
	return m.err
}

func TestRunSyncCore_Success_Verbose(t *testing.T) {
	var buf bytes.Buffer
	runner := &mockSyncRunner{}
	err := runSyncCore(context.Background(), runner, syncCoreOpts{
		rpc:    "http://localhost:26657",
		remote: "http://remote:26657",
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsSubstr(buf.String(), "Sync complete! Node is fully synced.") {
		t.Errorf("expected verbose completion message, got: %s", buf.String())
	}
}

func TestRunSyncCore_Success_Quiet(t *testing.T) {
	var buf bytes.Buffer
	runner := &mockSyncRunner{}
	err := runSyncCore(context.Background(), runner, syncCoreOpts{
		quiet: true,
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsSubstr(buf.String(), "Sync complete.") {
		t.Errorf("expected quiet completion message, got: %s", buf.String())
	}
	if containsSubstr(buf.String(), "fully synced") {
		t.Error("quiet mode should not contain verbose message")
	}
}

func TestRunSyncCore_SkipFinal(t *testing.T) {
	var buf bytes.Buffer
	runner := &mockSyncRunner{}
	err := runSyncCore(context.Background(), runner, syncCoreOpts{
		skipFinal: true,
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output with skipFinal, got: %s", buf.String())
	}
}

func TestRunSyncCore_SyncStuckError(t *testing.T) {
	var buf bytes.Buffer
	runner := &mockSyncRunner{err: syncmon.ErrSyncStuck}
	err := runSyncCore(context.Background(), runner, syncCoreOpts{}, &buf)
	if err == nil {
		t.Fatal("expected error")
	}
	var exitErr *exitcodes.ErrorWithCode
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exitcodes.ErrorWithCode, got: %T: %v", err, err)
	}
	if exitErr.Code != exitcodes.SyncStuck {
		t.Errorf("expected SyncStuck exit code, got: %d", exitErr.Code)
	}
}

func TestRunSyncCore_OtherError(t *testing.T) {
	var buf bytes.Buffer
	runner := &mockSyncRunner{err: fmt.Errorf("network timeout")}
	err := runSyncCore(context.Background(), runner, syncCoreOpts{}, &buf)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstr(err.Error(), "network timeout") {
		t.Errorf("expected network timeout error, got: %v", err)
	}
}

func TestRunSyncCore_EnvTimeout(t *testing.T) {
	t.Setenv("PNM_SYNC_STUCK_TIMEOUT", "5m")
	var buf bytes.Buffer
	runner := &mockSyncRunner{}
	_ = runSyncCore(context.Background(), runner, syncCoreOpts{}, &buf)
	if runner.opts.StuckTimeout.String() != "5m0s" {
		t.Errorf("expected 5m0s stuck timeout from env, got: %s", runner.opts.StuckTimeout)
	}
}

func TestRunSyncCore_EnvTimeout_Invalid(t *testing.T) {
	t.Setenv("PNM_SYNC_STUCK_TIMEOUT", "not-a-duration")
	var buf bytes.Buffer
	runner := &mockSyncRunner{}
	_ = runSyncCore(context.Background(), runner, syncCoreOpts{}, &buf)
	if runner.opts.StuckTimeout != 0 {
		t.Errorf("expected 0 stuck timeout for invalid env, got: %s", runner.opts.StuckTimeout)
	}
}

func TestRunSyncCore_ExplicitTimeout_OverridesEnv(t *testing.T) {
	t.Setenv("PNM_SYNC_STUCK_TIMEOUT", "10m")
	var buf bytes.Buffer
	runner := &mockSyncRunner{}
	_ = runSyncCore(context.Background(), runner, syncCoreOpts{
		stuckTimeout: 2 * 60_000_000_000, // 2m in nanoseconds
	}, &buf)
	if runner.opts.StuckTimeout.String() != "2m0s" {
		t.Errorf("expected 2m0s explicit timeout, got: %s", runner.opts.StuckTimeout)
	}
}

func TestRunSyncCore_PassesOptions(t *testing.T) {
	var buf bytes.Buffer
	runner := &mockSyncRunner{}
	_ = runSyncCore(context.Background(), runner, syncCoreOpts{
		rpc:      "http://local:26657",
		remote:   "http://remote:26657",
		logPath:  "/tmp/test.log",
		window:   50,
		compact:  true,
		quiet:    true,
		debug:    true,
	}, &buf)
	if runner.opts.LocalRPC != "http://local:26657" {
		t.Errorf("expected LocalRPC to be passed, got: %s", runner.opts.LocalRPC)
	}
	if runner.opts.RemoteRPC != "http://remote:26657" {
		t.Errorf("expected RemoteRPC to be passed, got: %s", runner.opts.RemoteRPC)
	}
	if runner.opts.LogPath != "/tmp/test.log" {
		t.Errorf("expected LogPath to be passed, got: %s", runner.opts.LogPath)
	}
	if runner.opts.Window != 50 {
		t.Errorf("expected Window 50, got: %d", runner.opts.Window)
	}
	if !runner.opts.Compact {
		t.Error("expected Compact true")
	}
	if !runner.opts.Quiet {
		t.Error("expected Quiet true")
	}
	if !runner.opts.Debug {
		t.Error("expected Debug true")
	}
}
