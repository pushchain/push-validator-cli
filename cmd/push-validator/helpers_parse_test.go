package main

import (
	"testing"
)

func TestParseDebugAddrField(t *testing.T) {
	sampleOutput := []byte(`Address: [106 211 108 238 ...]
Address (hex): 6AD36CEE5A9113907D5893135B32E3392CFBE94F
Bech32 Acc: push1dtfkemne22yusl2cn5y6lvewxwfk0a9rcs7rv6
Bech32 Val: pushvaloper1dtfkemne22yusl2cn5y6lvewxwfk0a9rhz45qp
Bech32 Con: pushvalcons1dtfkemne22yusl2cn5y6lvewxwfk0a9rjf8n0x
`)

	tests := []struct {
		name        string
		output      []byte
		fieldPrefix string
		want        string
		wantErr     bool
	}{
		{
			name:        "extract Bech32 Acc",
			output:      sampleOutput,
			fieldPrefix: "Bech32 Acc:",
			want:        "push1dtfkemne22yusl2cn5y6lvewxwfk0a9rcs7rv6",
		},
		{
			name:        "extract Address (hex)",
			output:      sampleOutput,
			fieldPrefix: "Address (hex):",
			want:        "6AD36CEE5A9113907D5893135B32E3392CFBE94F",
		},
		{
			name:        "extract Bech32 Val",
			output:      sampleOutput,
			fieldPrefix: "Bech32 Val:",
			want:        "pushvaloper1dtfkemne22yusl2cn5y6lvewxwfk0a9rhz45qp",
		},
		{
			name:        "extract Bech32 Con",
			output:      sampleOutput,
			fieldPrefix: "Bech32 Con:",
			want:        "pushvalcons1dtfkemne22yusl2cn5y6lvewxwfk0a9rjf8n0x",
		},
		{
			name:        "field not found",
			output:      sampleOutput,
			fieldPrefix: "Unknown Field:",
			wantErr:     true,
		},
		{
			name:        "empty output",
			output:      []byte(""),
			fieldPrefix: "Bech32 Acc:",
			wantErr:     true,
		},
		{
			name:        "malformed line - too few fields",
			output:      []byte("Bech32 Acc:\n"),
			fieldPrefix: "Bech32 Acc:",
			wantErr:     true,
		},
		{
			name:        "field with extra spaces",
			output:      []byte("Bech32 Acc:   push1abc123def456\n"),
			fieldPrefix: "Bech32 Acc:",
			want:        "push1abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDebugAddrField(tt.output, tt.fieldPrefix)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDebugAddrField() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Errorf("parseDebugAddrField() error = %v, want nil", err)
				return
			}
			if got != tt.want {
				t.Errorf("parseDebugAddrField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseKeysListJSON(t *testing.T) {
	validJSON := []byte(`[
		{"name": "validator-key", "address": "push1abc123"},
		{"name": "my-wallet", "address": "push1xyz789"},
		{"name": "backup-key", "address": "push1def456"}
	]`)

	tests := []struct {
		name    string
		output  []byte
		target  string
		want    string
		wantErr bool
	}{
		{
			name:   "match first key",
			output: validJSON,
			target: "push1abc123",
			want:   "validator-key",
		},
		{
			name:   "match second key",
			output: validJSON,
			target: "push1xyz789",
			want:   "my-wallet",
		},
		{
			name:   "match third key",
			output: validJSON,
			target: "push1def456",
			want:   "backup-key",
		},
		{
			name:    "no match",
			output:  validJSON,
			target:  "push1nonexistent",
			wantErr: true,
		},
		{
			name:    "empty list",
			output:  []byte(`[]`),
			target:  "push1abc123",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			output:  []byte(`not json`),
			target:  "push1abc123",
			wantErr: true,
		},
		{
			name:    "null JSON",
			output:  []byte(`null`),
			target:  "push1abc123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseKeysListJSON(tt.output, tt.target)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseKeysListJSON() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Errorf("parseKeysListJSON() error = %v, want nil", err)
				return
			}
			if got != tt.want {
				t.Errorf("parseKeysListJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseBinaryVersionOutput(t *testing.T) {
	tests := []struct {
		name   string
		output []byte
		want   string
	}{
		{
			name: "standard format",
			output: []byte(`name: pchaind
server_name: pchaind
version: v0.1.2-beta
commit: abc123def
build_tags: netgo,ledger
go: go1.21.0
`),
			want: "v0.1.2-beta",
		},
		{
			name:   "version on first line",
			output: []byte("version: v1.0.0\n"),
			want:   "v1.0.0",
		},
		{
			name: "version with leading spaces",
			output: []byte(`  version: v2.3.4
commit: deadbeef
`),
			want: "v2.3.4",
		},
		{
			name:   "empty output",
			output: []byte(""),
			want:   "",
		},
		{
			name:   "no version line",
			output: []byte("commit: abc123\nbuild_tags: netgo\n"),
			want:   "",
		},
		{
			name:   "version line with no colon value",
			output: []byte("version\n"),
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBinaryVersionOutput(tt.output)
			if got != tt.want {
				t.Errorf("parseBinaryVersionOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}
