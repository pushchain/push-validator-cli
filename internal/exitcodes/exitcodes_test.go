package exitcodes

import (
	"errors"
	"fmt"
	"testing"
)

// TestExitCodeConstants verifies all exit code constants have expected values
func TestExitCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"Success", Success, 0},
		{"GeneralError", GeneralError, 1},
		{"InvalidArgs", InvalidArgs, 2},
		{"PreconditionFailed", PreconditionFailed, 3},
		{"NetworkError", NetworkError, 4},
		{"ProcessError", ProcessError, 5},
		{"ValidationError", ValidationError, 6},
		{"SyncStuck", SyncStuck, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

// TestNewError tests NewError constructor
func TestNewError(t *testing.T) {
	tests := []struct {
		name        string
		code        int
		message     string
		wantCode    int
		wantMessage string
	}{
		{
			name:        "simple error",
			code:        InvalidArgs,
			message:     "invalid argument",
			wantCode:    InvalidArgs,
			wantMessage: "invalid argument",
		},
		{
			name:        "network error",
			code:        NetworkError,
			message:     "connection failed",
			wantCode:    NetworkError,
			wantMessage: "connection failed",
		},
		{
			name:        "custom code",
			code:        99,
			message:     "custom error",
			wantCode:    99,
			wantMessage: "custom error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.code, tt.message)
			if err.Code != tt.wantCode {
				t.Errorf("NewError() Code = %d, want %d", err.Code, tt.wantCode)
			}
			if err.Message != tt.wantMessage {
				t.Errorf("NewError() Message = %q, want %q", err.Message, tt.wantMessage)
			}
			if err.Cause != nil {
				t.Errorf("NewError() Cause = %v, want nil", err.Cause)
			}
			if err.Error() != tt.wantMessage {
				t.Errorf("NewError().Error() = %q, want %q", err.Error(), tt.wantMessage)
			}
		})
	}
}

// TestNewErrorf tests NewErrorf constructor with formatting
func TestNewErrorf(t *testing.T) {
	tests := []struct {
		name        string
		code        int
		format      string
		args        []interface{}
		wantCode    int
		wantMessage string
	}{
		{
			name:        "single arg",
			code:        InvalidArgs,
			format:      "invalid value: %s",
			args:        []interface{}{"test"},
			wantCode:    InvalidArgs,
			wantMessage: "invalid value: test",
		},
		{
			name:        "multiple args",
			code:        NetworkError,
			format:      "port %d on host %s is unreachable",
			args:        []interface{}{8080, "localhost"},
			wantCode:    NetworkError,
			wantMessage: "port 8080 on host localhost is unreachable",
		},
		{
			name:        "no args",
			code:        ProcessError,
			format:      "process failed",
			args:        []interface{}{},
			wantCode:    ProcessError,
			wantMessage: "process failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrorf(tt.code, tt.format, tt.args...)
			if err.Code != tt.wantCode {
				t.Errorf("NewErrorf() Code = %d, want %d", err.Code, tt.wantCode)
			}
			if err.Message != tt.wantMessage {
				t.Errorf("NewErrorf() Message = %q, want %q", err.Message, tt.wantMessage)
			}
			if err.Cause != nil {
				t.Errorf("NewErrorf() Cause = %v, want nil", err.Cause)
			}
			if err.Error() != tt.wantMessage {
				t.Errorf("NewErrorf().Error() = %q, want %q", err.Error(), tt.wantMessage)
			}
		})
	}
}

// TestWrapError tests WrapError constructor
func TestWrapError(t *testing.T) {
	baseErr := errors.New("base error")
	ioErr := fmt.Errorf("io error")

	tests := []struct {
		name        string
		code        int
		message     string
		cause       error
		wantCode    int
		wantMessage string
		wantError   string
	}{
		{
			name:        "wrap standard error",
			code:        NetworkError,
			message:     "connection failed",
			cause:       baseErr,
			wantCode:    NetworkError,
			wantMessage: "connection failed",
			wantError:   "connection failed: base error",
		},
		{
			name:        "wrap formatted error",
			code:        ProcessError,
			message:     "process start failed",
			cause:       ioErr,
			wantCode:    ProcessError,
			wantMessage: "process start failed",
			wantError:   "process start failed: io error",
		},
		{
			name:        "wrap nil error",
			code:        InvalidArgs,
			message:     "validation failed",
			cause:       nil,
			wantCode:    InvalidArgs,
			wantMessage: "validation failed",
			wantError:   "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WrapError(tt.code, tt.message, tt.cause)
			if err.Code != tt.wantCode {
				t.Errorf("WrapError() Code = %d, want %d", err.Code, tt.wantCode)
			}
			if err.Message != tt.wantMessage {
				t.Errorf("WrapError() Message = %q, want %q", err.Message, tt.wantMessage)
			}
			if err.Cause != tt.cause {
				t.Errorf("WrapError() Cause = %v, want %v", err.Cause, tt.cause)
			}
			if err.Error() != tt.wantError {
				t.Errorf("WrapError().Error() = %q, want %q", err.Error(), tt.wantError)
			}
		})
	}
}

// TestErrorWithCode_Error tests the Error() method
func TestErrorWithCode_Error(t *testing.T) {
	tests := []struct {
		name  string
		err   *ErrorWithCode
		want  string
	}{
		{
			name: "error without cause",
			err:  &ErrorWithCode{Code: InvalidArgs, Message: "missing flag"},
			want: "missing flag",
		},
		{
			name: "error with cause",
			err:  &ErrorWithCode{Code: NetworkError, Message: "request failed", Cause: errors.New("timeout")},
			want: "request failed: timeout",
		},
		{
			name: "error with nil cause",
			err:  &ErrorWithCode{Code: ProcessError, Message: "process error", Cause: nil},
			want: "process error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ErrorWithCode.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestErrorWithCode_Unwrap tests the Unwrap() method
func TestErrorWithCode_Unwrap(t *testing.T) {
	baseErr := errors.New("base error")

	tests := []struct {
		name string
		err  *ErrorWithCode
		want error
	}{
		{
			name: "unwrap with cause",
			err:  &ErrorWithCode{Code: InvalidArgs, Message: "wrapper", Cause: baseErr},
			want: baseErr,
		},
		{
			name: "unwrap without cause",
			err:  &ErrorWithCode{Code: NetworkError, Message: "no cause"},
			want: nil,
		},
		{
			name: "unwrap with nil cause",
			err:  &ErrorWithCode{Code: ProcessError, Message: "nil cause", Cause: nil},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Unwrap(); got != tt.want {
				t.Errorf("ErrorWithCode.Unwrap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestInvalidArgsError tests InvalidArgsError constructor
func TestInvalidArgsError(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"simple message", "missing required flag"},
		{"detailed message", "invalid value for --port: must be between 1-65535"},
		{"empty message", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InvalidArgsError(tt.message)
			if err.Code != InvalidArgs {
				t.Errorf("InvalidArgsError() Code = %d, want %d", err.Code, InvalidArgs)
			}
			if err.Message != tt.message {
				t.Errorf("InvalidArgsError() Message = %q, want %q", err.Message, tt.message)
			}
			if err.Error() != tt.message {
				t.Errorf("InvalidArgsError().Error() = %q, want %q", err.Error(), tt.message)
			}
		})
	}
}

// TestInvalidArgsErrorf tests InvalidArgsErrorf constructor
func TestInvalidArgsErrorf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{"single arg", "invalid flag: %s", []interface{}{"--port"}, "invalid flag: --port"},
		{"multiple args", "flag %s has invalid value: %d", []interface{}{"--port", 99999}, "flag --port has invalid value: 99999"},
		{"no args", "validation failed", []interface{}{}, "validation failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InvalidArgsErrorf(tt.format, tt.args...)
			if err.Code != InvalidArgs {
				t.Errorf("InvalidArgsErrorf() Code = %d, want %d", err.Code, InvalidArgs)
			}
			if err.Message != tt.want {
				t.Errorf("InvalidArgsErrorf() Message = %q, want %q", err.Message, tt.want)
			}
		})
	}
}

// TestPreconditionError tests PreconditionError constructor
func TestPreconditionError(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"not initialized", "node not initialized, run 'init' first"},
		{"already running", "node is already running"},
		{"config missing", "configuration file not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PreconditionError(tt.message)
			if err.Code != PreconditionFailed {
				t.Errorf("PreconditionError() Code = %d, want %d", err.Code, PreconditionFailed)
			}
			if err.Message != tt.message {
				t.Errorf("PreconditionError() Message = %q, want %q", err.Message, tt.message)
			}
		})
	}
}

// TestPreconditionErrorf tests PreconditionErrorf constructor
func TestPreconditionErrorf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{"file missing", "config file %s not found", []interface{}{"/path/to/config"}, "config file /path/to/config not found"},
		{"state mismatch", "expected state %s but got %s", []interface{}{"running", "stopped"}, "expected state running but got stopped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PreconditionErrorf(tt.format, tt.args...)
			if err.Code != PreconditionFailed {
				t.Errorf("PreconditionErrorf() Code = %d, want %d", err.Code, PreconditionFailed)
			}
			if err.Message != tt.want {
				t.Errorf("PreconditionErrorf() Message = %q, want %q", err.Message, tt.want)
			}
		})
	}
}

// TestNetworkErr tests NetworkErr constructor
func TestNetworkErr(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"connection timeout", "connection timeout after 30s"},
		{"dns failure", "DNS resolution failed for host.example.com"},
		{"connection refused", "connection refused on port 8545"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NetworkErr(tt.message)
			if err.Code != NetworkError {
				t.Errorf("NetworkErr() Code = %d, want %d", err.Code, NetworkError)
			}
			if err.Message != tt.message {
				t.Errorf("NetworkErr() Message = %q, want %q", err.Message, tt.message)
			}
		})
	}
}

// TestNetworkErrf tests NetworkErrf constructor
func TestNetworkErrf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{"timeout with duration", "request timeout after %ds", []interface{}{30}, "request timeout after 30s"},
		{"connection details", "failed to connect to %s:%d", []interface{}{"localhost", 8545}, "failed to connect to localhost:8545"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NetworkErrf(tt.format, tt.args...)
			if err.Code != NetworkError {
				t.Errorf("NetworkErrf() Code = %d, want %d", err.Code, NetworkError)
			}
			if err.Message != tt.want {
				t.Errorf("NetworkErrf() Message = %q, want %q", err.Message, tt.want)
			}
		})
	}
}

// TestProcessErr tests ProcessErr constructor
func TestProcessErr(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"start failure", "failed to start process"},
		{"permission denied", "permission denied: cannot kill process"},
		{"not found", "process not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProcessErr(tt.message)
			if err.Code != ProcessError {
				t.Errorf("ProcessErr() Code = %d, want %d", err.Code, ProcessError)
			}
			if err.Message != tt.message {
				t.Errorf("ProcessErr() Message = %q, want %q", err.Message, tt.message)
			}
		})
	}
}

// TestProcessErrf tests ProcessErrf constructor
func TestProcessErrf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{"pid details", "process %d not found", []interface{}{12345}, "process 12345 not found"},
		{"signal failure", "failed to send %s to process %d", []interface{}{"SIGTERM", 67890}, "failed to send SIGTERM to process 67890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProcessErrf(tt.format, tt.args...)
			if err.Code != ProcessError {
				t.Errorf("ProcessErrf() Code = %d, want %d", err.Code, ProcessError)
			}
			if err.Message != tt.want {
				t.Errorf("ProcessErrf() Message = %q, want %q", err.Message, tt.want)
			}
		})
	}
}

// TestValidationErr tests ValidationErr constructor
func TestValidationErr(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"invalid config", "invalid configuration: missing required field"},
		{"corrupted data", "data corruption detected in state file"},
		{"checksum mismatch", "checksum verification failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationErr(tt.message)
			if err.Code != ValidationError {
				t.Errorf("ValidationErr() Code = %d, want %d", err.Code, ValidationError)
			}
			if err.Message != tt.message {
				t.Errorf("ValidationErr() Message = %q, want %q", err.Message, tt.message)
			}
		})
	}
}

// TestValidationErrf tests ValidationErrf constructor
func TestValidationErrf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{"field validation", "field %s is required", []interface{}{"email"}, "field email is required"},
		{"range validation", "value %d out of range [%d-%d]", []interface{}{999, 1, 100}, "value 999 out of range [1-100]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationErrf(tt.format, tt.args...)
			if err.Code != ValidationError {
				t.Errorf("ValidationErrf() Code = %d, want %d", err.Code, ValidationError)
			}
			if err.Message != tt.want {
				t.Errorf("ValidationErrf() Message = %q, want %q", err.Message, tt.want)
			}
		})
	}
}

// TestCodeForError tests CodeForError function
func TestCodeForError(t *testing.T) {
	standardErr := errors.New("standard error")
	fmtErr := fmt.Errorf("formatted error")

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error",
			err:  nil,
			want: Success,
		},
		{
			name: "InvalidArgs error",
			err:  InvalidArgsError("invalid arg"),
			want: InvalidArgs,
		},
		{
			name: "PreconditionFailed error",
			err:  PreconditionError("not initialized"),
			want: PreconditionFailed,
		},
		{
			name: "NetworkError error",
			err:  NetworkErr("connection failed"),
			want: NetworkError,
		},
		{
			name: "ProcessError error",
			err:  ProcessErr("process failed"),
			want: ProcessError,
		},
		{
			name: "ValidationError error",
			err:  ValidationErr("validation failed"),
			want: ValidationError,
		},
		{
			name: "custom code",
			err:  NewError(99, "custom error"),
			want: 99,
		},
		{
			name: "SyncStuck code",
			err:  NewError(SyncStuck, "sync stuck"),
			want: SyncStuck,
		},
		{
			name: "standard error",
			err:  standardErr,
			want: GeneralError,
		},
		{
			name: "formatted error",
			err:  fmtErr,
			want: GeneralError,
		},
		{
			name: "wrapped ErrorWithCode",
			err:  WrapError(NetworkError, "network issue", standardErr),
			want: NetworkError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CodeForError(tt.err); got != tt.want {
				t.Errorf("CodeForError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// TestErrorChaining tests that errors can be properly chained and unwrapped
func TestErrorChaining(t *testing.T) {
	baseErr := errors.New("base error")
	wrappedErr := WrapError(NetworkError, "network failure", baseErr)

	// Test that wrappedErr implements error
	var _ error = wrappedErr

	// Test that Error() includes cause
	if wrappedErr.Error() != "network failure: base error" {
		t.Errorf("Error() = %q, want %q", wrappedErr.Error(), "network failure: base error")
	}

	// Test that Unwrap() returns the cause
	if unwrapped := wrappedErr.Unwrap(); unwrapped != baseErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, baseErr)
	}

	// Test that errors.Is works with wrapped errors
	if !errors.Is(wrappedErr, baseErr) {
		t.Errorf("errors.Is(wrappedErr, baseErr) = false, want true")
	}
}

// TestMultipleLevelWrapping tests wrapping ErrorWithCode with another ErrorWithCode
func TestMultipleLevelWrapping(t *testing.T) {
	baseErr := errors.New("io error")
	level1 := WrapError(ProcessError, "process failed", baseErr)
	level2 := WrapError(GeneralError, "operation failed", level1)

	// Level 2 should return level 1 when unwrapped
	if level2.Unwrap() != level1 {
		t.Errorf("level2.Unwrap() != level1")
	}

	// Level 1 should return base error when unwrapped
	if level1.Unwrap() != baseErr {
		t.Errorf("level1.Unwrap() != baseErr")
	}

	// errors.Is should work through multiple levels
	if !errors.Is(level2, baseErr) {
		t.Errorf("errors.Is(level2, baseErr) = false, want true")
	}

	// CodeForError should return the code from the outermost error
	if code := CodeForError(level2); code != GeneralError {
		t.Errorf("CodeForError(level2) = %d, want %d", code, GeneralError)
	}
}
