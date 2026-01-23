package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseChecksumFile(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:     "ValidFormat_TwoSpaces",
			input:    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  latest.tar.lz4",
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:     "ValidFormat_OneSpace",
			input:    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789 latest.tar.lz4",
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:     "ValidFormat_HashOnly",
			input:    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:     "ValidFormat_WithLeadingWhitespace",
			input:    "  abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  latest.tar.lz4",
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:     "ValidFormat_WithComment",
			input:    "# This is a comment\nabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  latest.tar.lz4",
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:     "ValidFormat_WithEmptyLines",
			input:    "\n\nabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  latest.tar.lz4\n\n",
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
		{
			name:        "InvalidFormat_TooShort",
			input:       "abc123  latest.tar.lz4",
			expectError: true,
		},
		{
			name:        "InvalidFormat_TooLong",
			input:       "abc123def456789012345678901234567890123456789012345678901234567890  latest.tar.lz4",
			expectError: true,
		},
		{
			name:        "InvalidFormat_NotHex",
			input:       "ghijklmnopqrstuvwxyz01234567890123456789012345678901234567890123  latest.tar.lz4",
			expectError: true,
		},
		{
			name:        "InvalidFormat_Empty",
			input:       "",
			expectError: true,
		},
		{
			name:        "InvalidFormat_OnlyWhitespace",
			input:       "   \n\n   ",
			expectError: true,
		},
		{
			name:        "InvalidFormat_OnlyComments",
			input:       "# Comment 1\n# Comment 2",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := parseChecksumFile(reader)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseChecksumFile() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseChecksumFile() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("parseChecksumFile() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestVerifyFile(t *testing.T) {
	t.Run("ValidChecksum", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")
		content := []byte("hello world")
		os.WriteFile(filePath, content, 0o644)

		// SHA256 of "hello world" is:
		expectedHash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

		err := verifyFile(filePath, expectedHash)
		if err != nil {
			t.Errorf("verifyFile() error = %v, expected nil", err)
		}
	})

	t.Run("InvalidChecksum", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")
		content := []byte("hello world")
		os.WriteFile(filePath, content, 0o644)

		wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"

		err := verifyFile(filePath, wrongHash)
		if err == nil {
			t.Error("verifyFile() expected error for mismatched hash, got nil")
		}
		if !strings.Contains(err.Error(), "hash mismatch") {
			t.Errorf("verifyFile() error = %v, expected 'hash mismatch'", err)
		}
	})

	t.Run("CaseInsensitiveHash", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")
		content := []byte("hello world")
		os.WriteFile(filePath, content, 0o644)

		// SHA256 of "hello world" in uppercase
		expectedHash := "B94D27B9934D3E08A52E52D7DA7DABFAC484EFE37A5380EE9088F7ACE2EFCDE9"

		err := verifyFile(filePath, expectedHash)
		if err != nil {
			t.Errorf("verifyFile() error = %v, expected nil (case insensitive)", err)
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "nonexistent.txt")

		err := verifyFile(filePath, "abc123")
		if err == nil {
			t.Error("verifyFile() expected error for nonexistent file, got nil")
		}
		if !strings.Contains(err.Error(), "open file") {
			t.Errorf("verifyFile() error = %v, expected 'open file'", err)
		}
	})

	t.Run("EmptyFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "empty.txt")
		os.WriteFile(filePath, []byte{}, 0o644)

		// SHA256 of empty string
		expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

		err := verifyFile(filePath, expectedHash)
		if err != nil {
			t.Errorf("verifyFile() error = %v, expected nil", err)
		}
	})

	t.Run("LargeFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "large.bin")

		// Create a file with 1MB of zeros
		content := make([]byte, 1024*1024)
		os.WriteFile(filePath, content, 0o644)

		// SHA256 of 1MB of zeros
		expectedHash := "30e14955ebf1352266dc2ff8067e68104607e750abb9d3b36582b8af909fcb58"

		err := verifyFile(filePath, expectedHash)
		if err != nil {
			t.Errorf("verifyFile() error = %v, expected nil", err)
		}
	})
}
