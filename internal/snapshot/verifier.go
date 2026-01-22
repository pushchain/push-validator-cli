package snapshot

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// parseChecksumFile parses a checksum file in the format:
// <sha256hash>  <filename>
// Returns the hash string.
func parseChecksumFile(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Format: <hash>  <filename> (two spaces between)
		// Or: <hash> <filename> (one space)
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			hash := parts[0]
			// Validate it looks like a SHA256 hash (64 hex chars)
			if len(hash) == 64 {
				if _, err := hex.DecodeString(hash); err == nil {
					return hash, nil
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read checksum file: %w", err)
	}

	return "", fmt.Errorf("no valid SHA256 hash found in checksum file")
}

// verifyFile computes the SHA256 hash of a file and compares it to expected.
func verifyFile(filePath, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}
