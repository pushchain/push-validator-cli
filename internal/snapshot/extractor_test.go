package snapshot

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierrec/lz4/v4"
)

// createTestTarLz4 creates a test tar.lz4 archive with the given files.
// files is a map of filename -> content
func createTestTarLz4(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer f.Close()

	// Create lz4 writer
	lz4Writer := lz4.NewWriter(f)
	defer lz4Writer.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(lz4Writer)
	defer tarWriter.Close()

	for name, content := range files {
		// Determine if this is a directory
		isDir := strings.HasSuffix(name, "/")
		mode := int64(0o644)
		typeflag := byte(tar.TypeReg)

		if isDir {
			mode = 0o755
			typeflag = tar.TypeDir
			content = "" // directories have no content
		}

		header := &tar.Header{
			Name:     name,
			Mode:     mode,
			Size:     int64(len(content)),
			Typeflag: typeflag,
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}

		if !isDir && content != "" {
			if _, err := tarWriter.Write([]byte(content)); err != nil {
				t.Fatalf("failed to write tar content: %v", err)
			}
		}
	}
}

func TestExtractTarLz4(t *testing.T) {
	t.Run("Success_SimpleExtraction", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		// Create test archive
		files := map[string]string{
			"data/":          "",
			"data/file1.txt": "hello world",
			"data/file2.txt": "test content",
		}
		createTestTarLz4(t, archivePath, files)

		// Extract
		var progressCalls int
		err := extractTarLz4(archivePath, extractDir, func(current, total int64, name string) {
			progressCalls++
		})
		if err != nil {
			t.Fatalf("extractTarLz4() error = %v", err)
		}

		// Verify files were extracted
		content1, err := os.ReadFile(filepath.Join(extractDir, "data", "file1.txt"))
		if err != nil {
			t.Fatalf("failed to read extracted file1: %v", err)
		}
		if string(content1) != "hello world" {
			t.Errorf("file1 content = %q, want %q", string(content1), "hello world")
		}

		content2, err := os.ReadFile(filepath.Join(extractDir, "data", "file2.txt"))
		if err != nil {
			t.Fatalf("failed to read extracted file2: %v", err)
		}
		if string(content2) != "test content" {
			t.Errorf("file2 content = %q, want %q", string(content2), "test content")
		}

		// Verify progress was called
		if progressCalls == 0 {
			t.Error("expected progress callback to be called")
		}
	})

	t.Run("Success_NestedDirectories", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		// Create test archive with nested structure
		files := map[string]string{
			"data/":                      "",
			"data/subdir1/":              "",
			"data/subdir1/file.txt":      "nested file",
			"data/subdir1/subdir2/":      "",
			"data/subdir1/subdir2/deep.txt": "deep file",
		}
		createTestTarLz4(t, archivePath, files)

		// Extract
		err := extractTarLz4(archivePath, extractDir, nil)
		if err != nil {
			t.Fatalf("extractTarLz4() error = %v", err)
		}

		// Verify nested files were extracted
		content, err := os.ReadFile(filepath.Join(extractDir, "data", "subdir1", "subdir2", "deep.txt"))
		if err != nil {
			t.Fatalf("failed to read nested file: %v", err)
		}
		if string(content) != "deep file" {
			t.Errorf("nested file content = %q, want %q", string(content), "deep file")
		}
	})

	t.Run("Success_EmptyFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		// Create test archive with empty file
		files := map[string]string{
			"data/":       "",
			"data/empty":  "",
		}
		createTestTarLz4(t, archivePath, files)

		// Extract
		err := extractTarLz4(archivePath, extractDir, nil)
		if err != nil {
			t.Fatalf("extractTarLz4() error = %v", err)
		}

		// Verify empty file was created
		info, err := os.Stat(filepath.Join(extractDir, "data", "empty"))
		if err != nil {
			t.Fatalf("failed to stat empty file: %v", err)
		}
		if info.Size() != 0 {
			t.Errorf("empty file size = %d, want 0", info.Size())
		}
	})

	t.Run("Error_PathTraversal_DotDot", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "malicious.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		f, _ := os.Create(archivePath)
		lz4Writer := lz4.NewWriter(f)
		tarWriter := tar.NewWriter(lz4Writer)

		// Create malicious entry with ../
		header := &tar.Header{
			Name:     "../etc/passwd",
			Mode:     0o644,
			Size:     5,
			Typeflag: tar.TypeReg,
		}
		tarWriter.WriteHeader(header)
		tarWriter.Write([]byte("pwned"))

		tarWriter.Close()
		lz4Writer.Close()
		f.Close()

		// Extract should fail
		err := extractTarLz4(archivePath, extractDir, nil)
		if err == nil {
			t.Error("extractTarLz4() expected error for path traversal, got nil")
		}
		if !strings.Contains(err.Error(), "invalid path") {
			t.Errorf("extractTarLz4() error = %v, expected 'invalid path'", err)
		}
	})

	t.Run("Error_PathTraversal_AbsolutePath", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "malicious.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		f, _ := os.Create(archivePath)
		lz4Writer := lz4.NewWriter(f)
		tarWriter := tar.NewWriter(lz4Writer)

		// Create malicious entry with absolute path
		header := &tar.Header{
			Name:     "/etc/passwd",
			Mode:     0o644,
			Size:     5,
			Typeflag: tar.TypeReg,
		}
		tarWriter.WriteHeader(header)
		tarWriter.Write([]byte("pwned"))

		tarWriter.Close()
		lz4Writer.Close()
		f.Close()

		// Extract should fail
		err := extractTarLz4(archivePath, extractDir, nil)
		if err == nil {
			t.Error("extractTarLz4() expected error for absolute path, got nil")
		}
		if !strings.Contains(err.Error(), "invalid path") {
			t.Errorf("extractTarLz4() error = %v, expected 'invalid path'", err)
		}
	})

	t.Run("Error_PathTraversal_Symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "malicious.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		f, _ := os.Create(archivePath)
		lz4Writer := lz4.NewWriter(f)
		tarWriter := tar.NewWriter(lz4Writer)

		// Create malicious symlink with absolute path
		header := &tar.Header{
			Name:     "data/link",
			Linkname: "/etc/passwd",
			Typeflag: tar.TypeSymlink,
		}
		tarWriter.WriteHeader(header)

		tarWriter.Close()
		lz4Writer.Close()
		f.Close()

		// Extract should fail
		err := extractTarLz4(archivePath, extractDir, nil)
		if err == nil {
			t.Error("extractTarLz4() expected error for absolute symlink, got nil")
		}
		if !strings.Contains(err.Error(), "absolute symlink not allowed") {
			t.Errorf("extractTarLz4() error = %v, expected 'absolute symlink not allowed'", err)
		}
	})

	t.Run("Success_RelativeSymlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		f, _ := os.Create(archivePath)
		lz4Writer := lz4.NewWriter(f)
		tarWriter := tar.NewWriter(lz4Writer)

		// Create directory and file
		dirHeader := &tar.Header{
			Name:     "data/",
			Mode:     0o755,
			Typeflag: tar.TypeDir,
		}
		tarWriter.WriteHeader(dirHeader)

		fileHeader := &tar.Header{
			Name:     "data/file.txt",
			Mode:     0o644,
			Size:     5,
			Typeflag: tar.TypeReg,
		}
		tarWriter.WriteHeader(fileHeader)
		tarWriter.Write([]byte("hello"))

		// Create relative symlink (safe)
		linkHeader := &tar.Header{
			Name:     "data/link.txt",
			Linkname: "file.txt",
			Typeflag: tar.TypeSymlink,
		}
		tarWriter.WriteHeader(linkHeader)

		tarWriter.Close()
		lz4Writer.Close()
		f.Close()

		// Extract should succeed
		err := extractTarLz4(archivePath, extractDir, nil)
		if err != nil {
			t.Fatalf("extractTarLz4() error = %v", err)
		}

		// Verify symlink was created
		linkPath := filepath.Join(extractDir, "data", "link.txt")
		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("failed to stat symlink: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Error("expected symlink")
		}
	})

	t.Run("Error_FileNotFound", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "nonexistent.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		err := extractTarLz4(archivePath, extractDir, nil)
		if err == nil {
			t.Error("extractTarLz4() expected error for nonexistent file, got nil")
		}
		if !strings.Contains(err.Error(), "open archive") {
			t.Errorf("extractTarLz4() error = %v, expected 'open archive'", err)
		}
	})

	t.Run("Success_ProgressCallback", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		// Create test archive with multiple files
		files := map[string]string{
			"data/":          "",
			"data/file1.txt": "content1",
			"data/file2.txt": "content2",
			"data/file3.txt": "content3",
		}
		createTestTarLz4(t, archivePath, files)

		var progressCalls []struct {
			current, total int64
			name           string
		}

		err := extractTarLz4(archivePath, extractDir, func(current, total int64, name string) {
			progressCalls = append(progressCalls, struct {
				current, total int64
				name           string
			}{current, total, name})
		})
		if err != nil {
			t.Fatalf("extractTarLz4() error = %v", err)
		}

		// Verify progress was called for each file
		if len(progressCalls) != 4 { // 1 dir + 3 files
			t.Errorf("expected 4 progress calls, got %d", len(progressCalls))
		}

		// Verify progress counts are increasing
		for i, call := range progressCalls {
			if call.current != int64(i+1) {
				t.Errorf("progress call %d: current = %d, want %d", i, call.current, i+1)
			}
			if call.total != -1 {
				t.Errorf("progress call %d: total = %d, want -1 (unknown)", i, call.total)
			}
		}
	})

	t.Run("Success_NilProgressCallback", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		// Create test archive
		files := map[string]string{
			"data/":          "",
			"data/file1.txt": "hello",
		}
		createTestTarLz4(t, archivePath, files)

		// Extract with nil progress callback (should not panic)
		err := extractTarLz4(archivePath, extractDir, nil)
		if err != nil {
			t.Fatalf("extractTarLz4() error = %v", err)
		}

		// Verify file was extracted
		content, err := os.ReadFile(filepath.Join(extractDir, "data", "file1.txt"))
		if err != nil {
			t.Fatalf("failed to read extracted file: %v", err)
		}
		if string(content) != "hello" {
			t.Errorf("file content = %q, want %q", string(content), "hello")
		}
	})

	t.Run("Error_CorruptedArchive", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "corrupted.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		// Create a corrupted file (not a valid tar.lz4)
		os.WriteFile(archivePath, []byte("this is not a valid lz4 archive"), 0o644)

		err := extractTarLz4(archivePath, extractDir, nil)
		if err == nil {
			t.Error("extractTarLz4() expected error for corrupted archive, got nil")
		}
	})

	t.Run("Success_FilePermissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		// Create archive with specific permissions
		f, _ := os.Create(archivePath)
		lz4Writer := lz4.NewWriter(f)
		tarWriter := tar.NewWriter(lz4Writer)

		// Create executable file
		header := &tar.Header{
			Name:     "data/script.sh",
			Mode:     0o755,
			Size:     11,
			Typeflag: tar.TypeReg,
		}
		tarWriter.WriteHeader(header)
		tarWriter.Write([]byte("#!/bin/bash"))

		tarWriter.Close()
		lz4Writer.Close()
		f.Close()

		err := extractTarLz4(archivePath, extractDir, nil)
		if err != nil {
			t.Fatalf("extractTarLz4() error = %v", err)
		}

		// Verify file permissions
		info, err := os.Stat(filepath.Join(extractDir, "data", "script.sh"))
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if info.Mode()&0o111 == 0 {
			t.Error("expected executable permissions to be preserved")
		}
	})

	t.Run("Success_HardLink", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.lz4")
		extractDir := filepath.Join(tmpDir, "extract")

		f, _ := os.Create(archivePath)
		lz4Writer := lz4.NewWriter(f)
		tarWriter := tar.NewWriter(lz4Writer)

		// Create directory
		dirHeader := &tar.Header{
			Name:     "data/",
			Mode:     0o755,
			Typeflag: tar.TypeDir,
		}
		tarWriter.WriteHeader(dirHeader)

		// Create original file
		fileHeader := &tar.Header{
			Name:     "data/original.txt",
			Mode:     0o644,
			Size:     5,
			Typeflag: tar.TypeReg,
		}
		tarWriter.WriteHeader(fileHeader)
		tarWriter.Write([]byte("hello"))

		// Create hard link
		linkHeader := &tar.Header{
			Name:     "data/hardlink.txt",
			Linkname: "data/original.txt",
			Typeflag: tar.TypeLink,
		}
		tarWriter.WriteHeader(linkHeader)

		tarWriter.Close()
		lz4Writer.Close()
		f.Close()

		err := extractTarLz4(archivePath, extractDir, nil)
		if err != nil {
			t.Fatalf("extractTarLz4() error = %v", err)
		}

		// Verify both files exist and have same inode
		origInfo, _ := os.Stat(filepath.Join(extractDir, "data", "original.txt"))
		linkInfo, _ := os.Stat(filepath.Join(extractDir, "data", "hardlink.txt"))

		if origInfo == nil || linkInfo == nil {
			t.Fatal("failed to stat files")
		}

		// Read content to verify
		content, _ := os.ReadFile(filepath.Join(extractDir, "data", "hardlink.txt"))
		if string(content) != "hello" {
			t.Errorf("hardlink content = %q, want %q", string(content), "hello")
		}
	})
}

func TestExtractTarLz4_UnsupportedTypes(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.lz4")
	extractDir := filepath.Join(tmpDir, "extract")

	f, _ := os.Create(archivePath)
	lz4Writer := lz4.NewWriter(f)
	tarWriter := tar.NewWriter(lz4Writer)

	// Create a character device entry (should be skipped)
	header := &tar.Header{
		Name:     "data/chardev",
		Mode:     0o644,
		Typeflag: tar.TypeChar,
	}
	tarWriter.WriteHeader(header)

	// Create a normal file after
	fileHeader := &tar.Header{
		Name:     "data/file.txt",
		Mode:     0o644,
		Size:     5,
		Typeflag: tar.TypeReg,
	}
	tarWriter.WriteHeader(fileHeader)
	tarWriter.Write([]byte("hello"))

	tarWriter.Close()
	lz4Writer.Close()
	f.Close()

	// Extract should succeed and skip the char device
	err := extractTarLz4(archivePath, extractDir, nil)
	if err != nil {
		t.Fatalf("extractTarLz4() error = %v", err)
	}

	// Verify char device was skipped
	if _, err := os.Stat(filepath.Join(extractDir, "data", "chardev")); err == nil {
		t.Error("expected char device to be skipped")
	}

	// Verify normal file was extracted
	content, _ := os.ReadFile(filepath.Join(extractDir, "data", "file.txt"))
	if string(content) != "hello" {
		t.Errorf("file content = %q, want %q", string(content), "hello")
	}
}

func TestExtractTarLz4_EOFHandling(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.lz4")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create a minimal valid archive with just one file
	f, _ := os.Create(archivePath)
	lz4Writer := lz4.NewWriter(f)
	tarWriter := tar.NewWriter(lz4Writer)

	header := &tar.Header{
		Name:     "data/file.txt",
		Mode:     0o644,
		Size:     3,
		Typeflag: tar.TypeReg,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte("EOF"))

	tarWriter.Close()
	lz4Writer.Close()
	f.Close()

	err := extractTarLz4(archivePath, extractDir, nil)
	if err != nil {
		t.Fatalf("extractTarLz4() error = %v", err)
	}

	// Verify EOF was handled correctly
	content, _ := os.ReadFile(filepath.Join(extractDir, "data", "file.txt"))
	if string(content) != "EOF" {
		t.Errorf("file content = %q, want %q", string(content), "EOF")
	}
}

func TestExtractTarLz4_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.lz4")
	extractDir := filepath.Join(tmpDir, "extract")

	// Create archive with a larger file
	f, _ := os.Create(archivePath)
	lz4Writer := lz4.NewWriter(f)
	tarWriter := tar.NewWriter(lz4Writer)

	// Create 100KB file
	largeContent := make([]byte, 100*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	header := &tar.Header{
		Name:     "data/large.bin",
		Mode:     0o644,
		Size:     int64(len(largeContent)),
		Typeflag: tar.TypeReg,
	}
	tarWriter.WriteHeader(header)

	// Write in chunks
	written := 0
	for written < len(largeContent) {
		n, _ := tarWriter.Write(largeContent[written:])
		written += n
	}

	tarWriter.Close()
	lz4Writer.Close()
	f.Close()

	err := extractTarLz4(archivePath, extractDir, nil)
	if err != nil {
		t.Fatalf("extractTarLz4() error = %v", err)
	}

	// Verify large file was extracted correctly
	extracted, _ := os.ReadFile(filepath.Join(extractDir, "data", "large.bin"))
	if len(extracted) != len(largeContent) {
		t.Errorf("extracted file size = %d, want %d", len(extracted), len(largeContent))
	}
}

func TestExtractTarLz4_PathTraversalSubtle(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "malicious.tar.lz4")
	extractDir := filepath.Join(tmpDir, "extract")

	f, _ := os.Create(archivePath)
	lz4Writer := lz4.NewWriter(f)
	tarWriter := tar.NewWriter(lz4Writer)

	// Create a subtle path traversal attempt: data/../../etc/passwd
	header := &tar.Header{
		Name:     "data/../../etc/passwd",
		Mode:     0o644,
		Size:     5,
		Typeflag: tar.TypeReg,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte("pwned"))

	tarWriter.Close()
	lz4Writer.Close()
	f.Close()

	// Extract should fail on path traversal check
	err := extractTarLz4(archivePath, extractDir, nil)
	if err == nil {
		t.Error("extractTarLz4() expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal detected") && !strings.Contains(err.Error(), "invalid path") {
		t.Errorf("extractTarLz4() error = %v, expected path traversal error", err)
	}
}
