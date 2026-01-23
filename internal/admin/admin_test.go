package admin

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestHome creates a complete test directory structure with dummy files
func setupTestHome(t *testing.T) string {
	t.Helper()
	homeDir := t.TempDir()

	// Create directory structure
	dirs := []string{
		filepath.Join(homeDir, "config"),
		filepath.Join(homeDir, "data"),
		filepath.Join(homeDir, "keyring-file"),
		filepath.Join(homeDir, "keyring-test"),
		filepath.Join(homeDir, "logs"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create config files
	configFiles := map[string]string{
		filepath.Join(homeDir, "config", "config.toml"):              "# config.toml content",
		filepath.Join(homeDir, "config", "app.toml"):                 "# app.toml content",
		filepath.Join(homeDir, "config", "genesis.json"):             `{"chain_id":"test"}`,
		filepath.Join(homeDir, "config", "priv_validator_key.json"):  `{"address":"test_validator"}`,
		filepath.Join(homeDir, "config", "node_key.json"):            `{"id":"test_node"}`,
		filepath.Join(homeDir, "config", "addrbook.json"):            `{"addrs":[]}`,
		filepath.Join(homeDir, "data", "priv_validator_state.json"): `{"height":"0"}`,
	}
	for path, content := range configFiles {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	// Create some dummy data files to simulate blockchain data
	dataFiles := []string{
		filepath.Join(homeDir, "data", "blockstore.db"),
		filepath.Join(homeDir, "data", "state.db"),
		filepath.Join(homeDir, "data", "tx_index.db"),
	}
	for _, path := range dataFiles {
		if err := os.WriteFile(path, []byte("dummy data"), 0o644); err != nil {
			t.Fatalf("failed to write data file %s: %v", path, err)
		}
	}

	// Create keyring files
	keyringFiles := []string{
		filepath.Join(homeDir, "keyring-file", "test.info"),
		filepath.Join(homeDir, "keyring-test", "test.info"),
	}
	for _, path := range keyringFiles {
		if err := os.WriteFile(path, []byte("keyring data"), 0o644); err != nil {
			t.Fatalf("failed to write keyring file %s: %v", path, err)
		}
	}

	return homeDir
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// dirIsEmpty checks if a directory exists and is empty
func dirIsEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) == 0
}

func TestReset(t *testing.T) {
	t.Run("successful reset with data removal", func(t *testing.T) {
		homeDir := setupTestHome(t)

		opts := ResetOptions{
			HomeDir: homeDir,
			BinPath: "pchaind",
		}

		err := Reset(opts)
		if err != nil {
			t.Fatalf("Reset failed: %v", err)
		}

		// Verify data directory was recreated and is empty
		dataDir := filepath.Join(homeDir, "data")
		if !fileExists(dataDir) {
			t.Error("data directory should exist after reset")
		}
		if !dirIsEmpty(dataDir) {
			t.Error("data directory should be empty after reset")
		}

		// Verify logs directory exists
		logsDir := filepath.Join(homeDir, "logs")
		if !fileExists(logsDir) {
			t.Error("logs directory should exist after reset")
		}

		// Verify validator keys are preserved
		privValKey := filepath.Join(homeDir, "config", "priv_validator_key.json")
		if !fileExists(privValKey) {
			t.Error("priv_validator_key.json should be preserved")
		}

		nodeKey := filepath.Join(homeDir, "config", "node_key.json")
		if !fileExists(nodeKey) {
			t.Error("node_key.json should be preserved")
		}

		// Verify keyring is preserved
		keyringFile := filepath.Join(homeDir, "keyring-file")
		if !fileExists(keyringFile) {
			t.Error("keyring-file should be preserved")
		}

		// Verify address book is removed when KeepAddrBook is false
		addrBook := filepath.Join(homeDir, "config", "addrbook.json")
		if !fileExists(addrBook) {
			t.Error("addrbook.json should still exist (not removed by Reset)")
		}
	})

	t.Run("reset with KeepAddrBook=true", func(t *testing.T) {
		homeDir := setupTestHome(t)

		// Write specific content to address book
		addrBookPath := filepath.Join(homeDir, "config", "addrbook.json")
		originalContent := `{"addrs":["addr1","addr2"]}`
		if err := os.WriteFile(addrBookPath, []byte(originalContent), 0o644); err != nil {
			t.Fatalf("failed to write address book: %v", err)
		}

		opts := ResetOptions{
			HomeDir:      homeDir,
			BinPath:      "pchaind",
			KeepAddrBook: true,
		}

		err := Reset(opts)
		if err != nil {
			t.Fatalf("Reset failed: %v", err)
		}

		// Verify address book was preserved
		if !fileExists(addrBookPath) {
			t.Error("addrbook.json should be preserved when KeepAddrBook=true")
		}

		// Verify content is the same
		content, err := os.ReadFile(addrBookPath)
		if err != nil {
			t.Fatalf("failed to read preserved address book: %v", err)
		}
		if string(content) != originalContent {
			t.Errorf("address book content mismatch: got %q, want %q", string(content), originalContent)
		}
	})

	t.Run("reset with KeepAddrBook=true but missing address book", func(t *testing.T) {
		homeDir := setupTestHome(t)

		// Remove address book
		addrBookPath := filepath.Join(homeDir, "config", "addrbook.json")
		if err := os.Remove(addrBookPath); err != nil {
			t.Fatalf("failed to remove address book: %v", err)
		}

		opts := ResetOptions{
			HomeDir:      homeDir,
			BinPath:      "pchaind",
			KeepAddrBook: true,
		}

		// Should not error even if address book doesn't exist
		err := Reset(opts)
		if err != nil {
			t.Fatalf("Reset should not fail when address book is missing: %v", err)
		}

		// Address book should not be recreated
		if fileExists(addrBookPath) {
			t.Error("addrbook.json should not be recreated if it didn't exist")
		}
	})

	t.Run("reset without HomeDir", func(t *testing.T) {
		opts := ResetOptions{
			BinPath: "pchaind",
		}

		err := Reset(opts)
		if err == nil {
			t.Error("Reset should fail when HomeDir is empty")
		}
		if !strings.Contains(err.Error(), "HomeDir required") {
			t.Errorf("expected 'HomeDir required' error, got: %v", err)
		}
	})

	t.Run("reset with non-existent HomeDir", func(t *testing.T) {
		opts := ResetOptions{
			HomeDir: "/nonexistent/path/that/does/not/exist",
			BinPath: "pchaind",
		}

		// Should not error, just create directories
		err := Reset(opts)
		if err != nil {
			t.Fatalf("Reset should handle non-existent HomeDir: %v", err)
		}
	})
}

func TestFullReset(t *testing.T) {
	t.Run("successful full reset removes everything", func(t *testing.T) {
		homeDir := setupTestHome(t)

		opts := FullResetOptions{
			HomeDir: homeDir,
			BinPath: "pchaind",
		}

		err := FullReset(opts)
		if err != nil {
			t.Fatalf("FullReset failed: %v", err)
		}

		// Verify data directory is empty
		dataDir := filepath.Join(homeDir, "data")
		if !fileExists(dataDir) {
			t.Error("data directory should exist after full reset")
		}
		if !dirIsEmpty(dataDir) {
			t.Error("data directory should be empty after full reset")
		}

		// Verify keyrings are removed
		keyringFile := filepath.Join(homeDir, "keyring-file")
		if fileExists(keyringFile) {
			t.Error("keyring-file should be removed after full reset")
		}

		keyringTest := filepath.Join(homeDir, "keyring-test")
		if fileExists(keyringTest) {
			t.Error("keyring-test should be removed after full reset")
		}

		// Verify validator keys are removed
		privValKey := filepath.Join(homeDir, "config", "priv_validator_key.json")
		if fileExists(privValKey) {
			t.Error("priv_validator_key.json should be removed after full reset")
		}

		nodeKey := filepath.Join(homeDir, "config", "node_key.json")
		if fileExists(nodeKey) {
			t.Error("node_key.json should be removed after full reset")
		}

		// Verify address book is removed
		addrBook := filepath.Join(homeDir, "config", "addrbook.json")
		if fileExists(addrBook) {
			t.Error("addrbook.json should be removed after full reset")
		}

		// Verify logs directory still exists
		logsDir := filepath.Join(homeDir, "logs")
		if !fileExists(logsDir) {
			t.Error("logs directory should exist after full reset")
		}

		// Verify config directory still exists (structure preserved)
		configDir := filepath.Join(homeDir, "config")
		if !fileExists(configDir) {
			t.Error("config directory should still exist")
		}
	})

	t.Run("full reset without HomeDir", func(t *testing.T) {
		opts := FullResetOptions{
			BinPath: "pchaind",
		}

		err := FullReset(opts)
		if err == nil {
			t.Error("FullReset should fail when HomeDir is empty")
		}
		if !strings.Contains(err.Error(), "HomeDir required") {
			t.Errorf("expected 'HomeDir required' error, got: %v", err)
		}
	})

	t.Run("full reset with default BinPath", func(t *testing.T) {
		homeDir := setupTestHome(t)

		opts := FullResetOptions{
			HomeDir: homeDir,
			// BinPath not set, should default to "pchaind"
		}

		err := FullReset(opts)
		if err != nil {
			t.Fatalf("FullReset should work with default BinPath: %v", err)
		}

		// Just verify it completed without error
		dataDir := filepath.Join(homeDir, "data")
		if !fileExists(dataDir) {
			t.Error("data directory should exist")
		}
	})

	t.Run("full reset with non-existent HomeDir", func(t *testing.T) {
		opts := FullResetOptions{
			HomeDir: "/nonexistent/path/that/does/not/exist",
			BinPath: "pchaind",
		}

		// Should not error, just create directories
		err := FullReset(opts)
		if err != nil {
			t.Fatalf("FullReset should handle non-existent HomeDir: %v", err)
		}
	})

	t.Run("full reset with partial missing files", func(t *testing.T) {
		homeDir := setupTestHome(t)

		// Remove some files before reset
		os.Remove(filepath.Join(homeDir, "config", "priv_validator_key.json"))
		os.Remove(filepath.Join(homeDir, "keyring-file", "test.info"))

		opts := FullResetOptions{
			HomeDir: homeDir,
			BinPath: "pchaind",
		}

		// Should not error even if some files don't exist
		err := FullReset(opts)
		if err != nil {
			t.Fatalf("FullReset should handle missing files gracefully: %v", err)
		}
	})
}

func TestBackup(t *testing.T) {
	t.Run("successful backup creation", func(t *testing.T) {
		homeDir := setupTestHome(t)

		opts := BackupOptions{
			HomeDir: homeDir,
		}

		backupPath, err := Backup(opts)
		if err != nil {
			t.Fatalf("Backup failed: %v", err)
		}

		// Verify backup file was created
		if !fileExists(backupPath) {
			t.Errorf("backup file should exist at %s", backupPath)
		}

		// Verify backup is in default location
		expectedDir := filepath.Join(homeDir, "backups")
		if !strings.HasPrefix(backupPath, expectedDir) {
			t.Errorf("backup should be in %s, got %s", expectedDir, backupPath)
		}

		// Verify filename format
		if !strings.HasPrefix(filepath.Base(backupPath), "backup-") {
			t.Errorf("backup filename should start with 'backup-', got %s", filepath.Base(backupPath))
		}
		if !strings.HasSuffix(backupPath, ".tar.gz") {
			t.Errorf("backup filename should end with '.tar.gz', got %s", backupPath)
		}

		// Verify backup contents
		verifyBackupContents(t, backupPath, []string{
			"config/config.toml",
			"config/app.toml",
			"config/genesis.json",
			"data/priv_validator_state.json",
		})
	})

	t.Run("backup with custom OutDir", func(t *testing.T) {
		homeDir := setupTestHome(t)
		customOutDir := filepath.Join(homeDir, "custom", "backups")

		opts := BackupOptions{
			HomeDir: homeDir,
			OutDir:  customOutDir,
		}

		backupPath, err := Backup(opts)
		if err != nil {
			t.Fatalf("Backup with custom OutDir failed: %v", err)
		}

		// Verify backup is in custom location
		if !strings.HasPrefix(backupPath, customOutDir) {
			t.Errorf("backup should be in %s, got %s", customOutDir, backupPath)
		}

		if !fileExists(backupPath) {
			t.Errorf("backup file should exist at %s", backupPath)
		}
	})

	t.Run("backup without HomeDir", func(t *testing.T) {
		opts := BackupOptions{}

		_, err := Backup(opts)
		if err == nil {
			t.Error("Backup should fail when HomeDir is empty")
		}
		if !strings.Contains(err.Error(), "HomeDir required") {
			t.Errorf("expected 'HomeDir required' error, got: %v", err)
		}
	})

	t.Run("backup with missing files", func(t *testing.T) {
		homeDir := setupTestHome(t)

		// Remove some files that would be backed up
		os.Remove(filepath.Join(homeDir, "config", "app.toml"))
		os.Remove(filepath.Join(homeDir, "data", "priv_validator_state.json"))

		opts := BackupOptions{
			HomeDir: homeDir,
		}

		// Should still succeed, just skip missing files
		backupPath, err := Backup(opts)
		if err != nil {
			t.Fatalf("Backup should succeed even with missing files: %v", err)
		}

		// Verify backup was created
		if !fileExists(backupPath) {
			t.Errorf("backup file should exist at %s", backupPath)
		}

		// Verify only existing files are in backup
		files := extractBackupFileList(t, backupPath)
		for _, file := range files {
			if strings.Contains(file, "app.toml") {
				t.Errorf("app.toml should not be in backup (was deleted)")
			}
			if strings.Contains(file, "priv_validator_state.json") {
				t.Errorf("priv_validator_state.json should not be in backup (was deleted)")
			}
		}
	})

	t.Run("backup with non-existent HomeDir", func(t *testing.T) {
		nonExistentDir := filepath.Join(t.TempDir(), "nonexistent")

		opts := BackupOptions{
			HomeDir: nonExistentDir,
		}

		// Should create backup dir but backup will be mostly empty
		backupPath, err := Backup(opts)
		if err != nil {
			t.Fatalf("Backup should handle non-existent HomeDir: %v", err)
		}

		// Verify backup was created (even if empty)
		if !fileExists(backupPath) {
			t.Errorf("backup file should exist at %s", backupPath)
		}
	})

	t.Run("backup creates valid archive", func(t *testing.T) {
		homeDir := setupTestHome(t)

		opts := BackupOptions{
			HomeDir: homeDir,
		}

		// Create backup
		backupPath, err := Backup(opts)
		if err != nil {
			t.Fatalf("Backup failed: %v", err)
		}

		// Verify it's a valid tar.gz by extracting it
		extractDir := filepath.Join(t.TempDir(), "extracted")
		if err := os.MkdirAll(extractDir, 0o755); err != nil {
			t.Fatalf("failed to create extract dir: %v", err)
		}

		// Extract the backup
		f, err := os.Open(backupPath)
		if err != nil {
			t.Fatalf("failed to open backup: %v", err)
		}
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer gz.Close()

		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("failed to read tar entry: %v", err)
			}

			// Verify we can read the entry
			targetPath := filepath.Join(extractDir, hdr.Name)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				t.Fatalf("failed to create dir for %s: %v", targetPath, err)
			}

			outFile, err := os.Create(targetPath)
			if err != nil {
				t.Fatalf("failed to create file %s: %v", targetPath, err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				t.Fatalf("failed to write file %s: %v", targetPath, err)
			}
			outFile.Close()
		}

		// Verify extracted files match originals
		configToml := filepath.Join(extractDir, "config", "config.toml")
		if !fileExists(configToml) {
			t.Error("extracted backup should contain config/config.toml")
		}
	})
}

func TestAddFile(t *testing.T) {
	t.Run("add regular file", func(t *testing.T) {
		homeDir := t.TempDir()
		testFile := filepath.Join(homeDir, "test.txt")
		testContent := "test content"
		if err := os.WriteFile(testFile, []byte(testContent), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Create tar writer
		tarFile := filepath.Join(homeDir, "test.tar")
		f, err := os.Create(tarFile)
		if err != nil {
			t.Fatalf("failed to create tar file: %v", err)
		}
		defer f.Close()

		tw := tar.NewWriter(f)
		defer tw.Close()

		// Add file
		err = addFile(tw, testFile, homeDir)
		if err != nil {
			t.Fatalf("addFile failed: %v", err)
		}

		// Close tar to flush
		tw.Close()
		f.Close()

		// Verify tar contents
		f2, err := os.Open(tarFile)
		if err != nil {
			t.Fatalf("failed to open tar file: %v", err)
		}
		defer f2.Close()

		tr := tar.NewReader(f2)
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}

		if hdr.Name != "test.txt" {
			t.Errorf("expected name 'test.txt', got %q", hdr.Name)
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("failed to read tar content: %v", err)
		}
		if string(content) != testContent {
			t.Errorf("expected content %q, got %q", testContent, string(content))
		}
	})

	t.Run("add non-existent file", func(t *testing.T) {
		homeDir := t.TempDir()
		testFile := filepath.Join(homeDir, "nonexistent.txt")

		tarFile := filepath.Join(homeDir, "test.tar")
		f, err := os.Create(tarFile)
		if err != nil {
			t.Fatalf("failed to create tar file: %v", err)
		}
		defer f.Close()

		tw := tar.NewWriter(f)
		defer tw.Close()

		// Should return error for non-existent file
		err = addFile(tw, testFile, homeDir)
		if err == nil {
			t.Error("addFile should fail for non-existent file")
		}
	})

	t.Run("add directory should be skipped", func(t *testing.T) {
		homeDir := t.TempDir()
		testDir := filepath.Join(homeDir, "testdir")
		if err := os.MkdirAll(testDir, 0o755); err != nil {
			t.Fatalf("failed to create test dir: %v", err)
		}

		tarFile := filepath.Join(homeDir, "test.tar")
		f, err := os.Create(tarFile)
		if err != nil {
			t.Fatalf("failed to create tar file: %v", err)
		}
		defer f.Close()

		tw := tar.NewWriter(f)
		defer tw.Close()

		// Should not error, but should not add directory
		err = addFile(tw, testDir, homeDir)
		if err != nil {
			t.Fatalf("addFile should handle directories: %v", err)
		}

		// Close and verify tar is empty
		tw.Close()
		f.Close()

		f2, err := os.Open(tarFile)
		if err != nil {
			t.Fatalf("failed to open tar file: %v", err)
		}
		defer f2.Close()

		tr := tar.NewReader(f2)
		_, err = tr.Next()
		if err != io.EOF {
			t.Error("tar should be empty (directory should not be added)")
		}
	})

	t.Run("add file with nested path", func(t *testing.T) {
		homeDir := t.TempDir()
		nestedDir := filepath.Join(homeDir, "a", "b", "c")
		if err := os.MkdirAll(nestedDir, 0o755); err != nil {
			t.Fatalf("failed to create nested dir: %v", err)
		}

		testFile := filepath.Join(nestedDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("nested content"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		tarFile := filepath.Join(homeDir, "test.tar")
		f, err := os.Create(tarFile)
		if err != nil {
			t.Fatalf("failed to create tar file: %v", err)
		}
		defer f.Close()

		tw := tar.NewWriter(f)
		defer tw.Close()

		err = addFile(tw, testFile, homeDir)
		if err != nil {
			t.Fatalf("addFile failed: %v", err)
		}

		tw.Close()
		f.Close()

		// Verify relative path in tar
		f2, err := os.Open(tarFile)
		if err != nil {
			t.Fatalf("failed to open tar file: %v", err)
		}
		defer f2.Close()

		tr := tar.NewReader(f2)
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}

		expectedPath := filepath.Join("a", "b", "c", "test.txt")
		if hdr.Name != expectedPath {
			t.Errorf("expected name %q, got %q", expectedPath, hdr.Name)
		}
	})
}

// Helper functions

// verifyBackupContents extracts and verifies expected files are in the backup
func verifyBackupContents(t *testing.T, backupPath string, expectedFiles []string) {
	t.Helper()

	files := extractBackupFileList(t, backupPath)

	for _, expected := range expectedFiles {
		found := false
		for _, file := range files {
			if file == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file %q not found in backup, got files: %v", expected, files)
		}
	}
}

// extractBackupFileList returns list of files in a tar.gz backup
func extractBackupFileList(t *testing.T, backupPath string) []string {
	t.Helper()

	f, err := os.Open(backupPath)
	if err != nil {
		t.Fatalf("failed to open backup file: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}
		files = append(files, hdr.Name)
	}

	return files
}
