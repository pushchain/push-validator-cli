package snapshot

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pierrec/lz4/v4"
)

// ExtractProgressFunc reports extraction progress.
// current: files extracted so far
// total: total files (-1 if unknown)
// name: current file being extracted
type ExtractProgressFunc func(current, total int64, name string)

// extractTarLz4 extracts a tar.lz4 archive to the destination directory.
// The archive is expected to contain a "data/" directory at the root.
func extractTarLz4(archivePath, destDir string, progress ExtractProgressFunc) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	// Create lz4 decompressor
	lz4Reader := lz4.NewReader(f)

	// Create tar reader
	tarReader := tar.NewReader(lz4Reader)

	var fileCount int64
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		// Security: prevent path traversal attacks
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || strings.HasPrefix(cleanName, "/") {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		// Construct target path
		targetPath := filepath.Join(destDir, cleanName)

		// Verify the path is still within destDir after joining
		if !strings.HasPrefix(targetPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("path traversal detected: %s", header.Name)
		}

		fileCount++
		if progress != nil {
			progress(fileCount, -1, cleanName)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create dir %s: %w", cleanName, err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent dir for %s: %w", cleanName, err)
			}

			// Create file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", cleanName, err)
			}

			// Copy contents
			written, copyErr := io.Copy(outFile, tarReader)
			if copyErr != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", cleanName, copyErr)
			}

			// Verify all bytes were written
			if header.Size > 0 && written != header.Size {
				outFile.Close()
				return fmt.Errorf("incomplete extraction of %s: wrote %d of %d bytes (disk full?)", cleanName, written, header.Size)
			}

			if err := outFile.Close(); err != nil {
				return fmt.Errorf("close file %s: %w", cleanName, err)
			}

		case tar.TypeSymlink:
			// Security: validate symlink target
			linkTarget := header.Linkname
			if filepath.IsAbs(linkTarget) {
				return fmt.Errorf("absolute symlink not allowed: %s -> %s", cleanName, linkTarget)
			}

			// Remove existing file/link if it exists
			os.Remove(targetPath)

			if err := os.Symlink(linkTarget, targetPath); err != nil {
				return fmt.Errorf("create symlink %s: %w", cleanName, err)
			}

		case tar.TypeLink:
			// Hard link
			linkTarget := filepath.Join(destDir, header.Linkname)
			if err := os.Link(linkTarget, targetPath); err != nil {
				return fmt.Errorf("create hard link %s: %w", cleanName, err)
			}

		default:
			// Skip other types (char devices, block devices, etc.)
			continue
		}
	}

	return nil
}
