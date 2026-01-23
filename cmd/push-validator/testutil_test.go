package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pushchain/push-validator-cli/internal/config"
	"github.com/pushchain/push-validator-cli/internal/node"
	"github.com/pushchain/push-validator-cli/internal/process"
	"github.com/pushchain/push-validator-cli/internal/validator"
)

// errMock is a generic error for test assertions.
var errMock = errors.New("mock error")

// mockSupervisor implements process.Supervisor for testing.
type mockSupervisor struct {
	running bool
	pid     int
	uptime  time.Duration
	logPath string
	stopErr error
	startPID int
	startErr error
}

func (m *mockSupervisor) Start(opts process.StartOpts) (int, error) {
	if m.startErr != nil {
		return 0, m.startErr
	}
	m.running = true
	return m.startPID, nil
}

func (m *mockSupervisor) Stop() error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.running = false
	return nil
}

func (m *mockSupervisor) Restart(opts process.StartOpts) (int, error) {
	if err := m.Stop(); err != nil {
		return 0, err
	}
	return m.Start(opts)
}

func (m *mockSupervisor) IsRunning() bool { return m.running }

func (m *mockSupervisor) PID() (int, bool) {
	if m.running && m.pid > 0 {
		return m.pid, true
	}
	return 0, false
}

func (m *mockSupervisor) Uptime() (time.Duration, bool) {
	if m.running {
		return m.uptime, true
	}
	return 0, false
}

func (m *mockSupervisor) LogPath() string { return m.logPath }

// mockNodeClient implements node.Client for testing.
type mockNodeClient struct {
	status    node.Status
	statusErr error
	peers     []node.Peer
	peersErr  error
}

func (m *mockNodeClient) Status(ctx context.Context) (node.Status, error) {
	return m.status, m.statusErr
}

func (m *mockNodeClient) RemoteStatus(ctx context.Context, baseURL string) (node.Status, error) {
	return m.status, m.statusErr
}

func (m *mockNodeClient) Peers(ctx context.Context) ([]node.Peer, error) {
	return m.peers, m.peersErr
}

func (m *mockNodeClient) SubscribeHeaders(ctx context.Context) (<-chan node.Header, error) {
	ch := make(chan node.Header)
	close(ch)
	return ch, nil
}

// mockValidator implements validator.Service for testing.
type mockValidator struct {
	balanceResult   string
	balanceErr      error
	isValidatorRes  bool
	isValidatorErr  error
	registerResult  string
	registerErr     error
	unjailResult    string
	unjailErr       error
	withdrawResult  string
	withdrawErr     error
	delegateResult  string
	delegateErr     error
	ensureKeyResult validator.KeyInfo
	ensureKeyErr    error
	importKeyResult validator.KeyInfo
	importKeyErr    error
	evmAddrResult   string
	evmAddrErr      error
}

func (m *mockValidator) Balance(ctx context.Context, addr string) (string, error) {
	return m.balanceResult, m.balanceErr
}

func (m *mockValidator) IsValidator(ctx context.Context, addr string) (bool, error) {
	return m.isValidatorRes, m.isValidatorErr
}

func (m *mockValidator) Register(ctx context.Context, args validator.RegisterArgs) (string, error) {
	return m.registerResult, m.registerErr
}

func (m *mockValidator) Unjail(ctx context.Context, keyName string) (string, error) {
	return m.unjailResult, m.unjailErr
}

func (m *mockValidator) WithdrawRewards(ctx context.Context, validatorAddr string, keyName string, includeCommission bool) (string, error) {
	return m.withdrawResult, m.withdrawErr
}

func (m *mockValidator) Delegate(ctx context.Context, args validator.DelegateArgs) (string, error) {
	return m.delegateResult, m.delegateErr
}

func (m *mockValidator) EnsureKey(ctx context.Context, name string) (validator.KeyInfo, error) {
	return m.ensureKeyResult, m.ensureKeyErr
}

func (m *mockValidator) ImportKey(ctx context.Context, name string, mnemonic string) (validator.KeyInfo, error) {
	return m.importKeyResult, m.importKeyErr
}

func (m *mockValidator) GetEVMAddress(ctx context.Context, addr string) (string, error) {
	return m.evmAddrResult, m.evmAddrErr
}

// mockRunner implements CommandRunner for testing.
type mockRunner struct {
	outputs map[string][]byte // key: "name arg1 arg2", value: output
	errors  map[string]error
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		outputs: make(map[string][]byte),
		errors:  make(map[string]error),
	}
}

func (m *mockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	key := name
	for _, a := range args {
		key += " " + a
	}
	if err, ok := m.errors[key]; ok {
		return nil, err
	}
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("mock: no output configured for %q", key)
}

// mockFetcher implements ValidatorFetcher for testing.
type mockFetcher struct {
	myValidator     validator.MyValidatorInfo
	myValidatorErr  error
	allValidators   validator.ValidatorList
	allValidatorsErr error
	commission      string
	outstanding     string
	rewardsErr      error
}

func (m *mockFetcher) GetMyValidator(ctx context.Context, cfg config.Config) (validator.MyValidatorInfo, error) {
	return m.myValidator, m.myValidatorErr
}

func (m *mockFetcher) GetAllValidators(ctx context.Context, cfg config.Config) (validator.ValidatorList, error) {
	return m.allValidators, m.allValidatorsErr
}

func (m *mockFetcher) GetRewards(ctx context.Context, cfg config.Config, addr string) (commission, outstanding string, err error) {
	return m.commission, m.outstanding, m.rewardsErr
}

// containsSubstr checks if s contains substr.
func containsSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// testCfg returns a minimal config for testing.
func testCfg() config.Config {
	return config.Config{
		ChainID:        "push_42101-1",
		HomeDir:        "/tmp/test-pchain",
		GenesisDomain:  "donut.rpc.push.org",
		KeyringBackend: "test",
		RPCLocal:       "http://127.0.0.1:26657",
		Denom:          "upc",
	}
}

// mockPrompter is a configurable prompter for testing.
// It returns responses in order and can be configured as interactive or not.
type mockPrompter struct {
	responses   []string
	interactive bool
	callIndex   int
}

func (p *mockPrompter) ReadLine(prompt string) (string, error) {
	if p.callIndex >= len(p.responses) {
		return "", fmt.Errorf("no more responses configured")
	}
	resp := p.responses[p.callIndex]
	p.callIndex++
	return resp, nil
}

func (p *mockPrompter) IsInteractive() bool {
	return p.interactive
}

// mockDashboardRunner implements DashboardRunner for tests.
type mockDashboardRunner struct {
	runErr error
	called bool
}

func (m *mockDashboardRunner) Run(cfg config.Config) error {
	m.called = true
	return m.runErr
}
