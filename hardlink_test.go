package archives

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestHardlinkDetection verifies that hardlinks are detected and properly
// tagged when gathering files from disk.
func TestHardlinkDetection(t *testing.T) {
	// Create a temporary directory with hardlinked files
	tmpDir := t.TempDir()

	// Create original file
	originalPath := filepath.Join(tmpDir, "original.txt")
	content := []byte("test content for hardlink")
	if err := os.WriteFile(originalPath, content, 0644); err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	// Create hardlink
	hardlinkPath := filepath.Join(tmpDir, "hardlink.txt")
	if err := os.Link(originalPath, hardlinkPath); err != nil {
		t.Fatalf("Failed to create hardlink: %v", err)
	}

	// Verify they are actually hardlinked (same inode)
	origInfo, _ := os.Stat(originalPath)
	linkInfo, _ := os.Stat(hardlinkPath)
	origStat := origInfo.Sys().(*syscall.Stat_t)
	linkStat := linkInfo.Sys().(*syscall.Stat_t)

	if origStat.Ino != linkStat.Ino {
		t.Fatalf("Files are not hardlinked: orig inode=%d, link inode=%d", origStat.Ino, linkStat.Ino)
	}

	// Gather files using FilesFromDisk
	ctx := context.Background()
	files, err := FilesFromDisk(ctx, nil, map[string]string{tmpDir + string(filepath.Separator): ""})
	if err != nil {
		t.Fatalf("FilesFromDisk failed: %v", err)
	}

	// Should have 2 files (no directory, just the files)
	if len(files) != 2 {
		for i, f := range files {
			t.Logf("File %d: Name=%s, NameInArchive=%s, LinkTarget=%q, IsRegular=%v",
				i, f.Name(), f.NameInArchive, f.LinkTarget, f.Mode().IsRegular())
		}
		t.Fatalf("Expected 2 files, got %d", len(files))
	}

	// Find the hardlink entry (the second occurrence based on walk order)
	// Note: The files may be walked in any order (usually alphabetical)
	var originalEntry, hardlinkEntry *FileInfo
	for i := range files {
		t.Logf("File %d: Name=%s, NameInArchive=%s, LinkTarget=%q, IsRegular=%v, Nlink=%d",
			i, files[i].Name(), files[i].NameInArchive, files[i].LinkTarget, files[i].Mode().IsRegular(),
			files[i].Sys().(*syscall.Stat_t).Nlink)
		if files[i].LinkTarget == "" {
			originalEntry = &files[i]
		} else {
			hardlinkEntry = &files[i]
		}
	}

	if originalEntry == nil {
		t.Fatal("Could not find original (first occurrence) entry")
	}
	if hardlinkEntry == nil {
		t.Fatal("Could not find hardlink (second occurrence) entry")
	}

	// Verify LinkTarget points to the first occurrence
	if hardlinkEntry.LinkTarget != originalEntry.NameInArchive {
		t.Errorf("LinkTarget = %q, want %q", hardlinkEntry.LinkTarget, originalEntry.NameInArchive)
	}
}

// TestHardlinkInTarArchive verifies that hardlinks are correctly written
// to tar archives with TypeLink headers.
func TestHardlinkInTarArchive(t *testing.T) {
	// Create a temporary directory with hardlinked files
	tmpDir := t.TempDir()

	// Create original file
	originalPath := filepath.Join(tmpDir, "file1.txt")
	content := []byte("shared content")
	if err := os.WriteFile(originalPath, content, 0644); err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	// Create hardlinks
	link2Path := filepath.Join(tmpDir, "file2.txt")
	link3Path := filepath.Join(tmpDir, "file3.txt")
	if err := os.Link(originalPath, link2Path); err != nil {
		t.Fatalf("Failed to create hardlink 2: %v", err)
	}
	if err := os.Link(originalPath, link3Path); err != nil {
		t.Fatalf("Failed to create hardlink 3: %v", err)
	}

	// Create tar archive
	archivePath := filepath.Join(t.TempDir(), "test.tar")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}
	defer archiveFile.Close()

	// Gather files and create archive
	ctx := context.Background()
	files, err := FilesFromDisk(ctx, nil, map[string]string{tmpDir: "testdir"})
	if err != nil {
		t.Fatalf("FilesFromDisk failed: %v", err)
	}

	format := Tar{}
	if err := format.Archive(ctx, archiveFile, files); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}
	archiveFile.Close()

	// Read back the archive and verify hardlinks
	archiveFile, err = os.Open(archivePath)
	if err != nil {
		t.Fatalf("Failed to open archive: %v", err)
	}
	defer archiveFile.Close()

	tr := tar.NewReader(archiveFile)

	var regularFiles, hardlinks int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read tar header: %v", err)
		}

		switch hdr.Typeflag {
		case tar.TypeReg:
			regularFiles++
			// Should be file1.txt
			if hdr.Name != "testdir/file1.txt" {
				t.Errorf("Unexpected regular file: %s", hdr.Name)
			}
			// Should have content
			if hdr.Size == 0 {
				t.Error("Regular file has zero size")
			}
		case tar.TypeLink:
			hardlinks++
			// Should be file2.txt or file3.txt
			if hdr.Name != "testdir/file2.txt" && hdr.Name != "testdir/file3.txt" {
				t.Errorf("Unexpected hardlink: %s", hdr.Name)
			}
			// Should have zero size
			if hdr.Size != 0 {
				t.Errorf("Hardlink %s has non-zero size: %d", hdr.Name, hdr.Size)
			}
			// Should point to original
			if hdr.Linkname != "testdir/file1.txt" {
				t.Errorf("Hardlink %s points to %s, want testdir/file1.txt", hdr.Name, hdr.Linkname)
			}
		}
	}

	// Verify counts
	if regularFiles != 1 {
		t.Errorf("Expected 1 regular file, got %d", regularFiles)
	}
	if hardlinks != 2 {
		t.Errorf("Expected 2 hardlinks, got %d", hardlinks)
	}
}

// TestHardlinkExtraction verifies that hardlinks can be extracted and
// the LinkTarget field is properly populated.
func TestHardlinkExtraction(t *testing.T) {
	// Create a temporary directory with hardlinked files
	srcDir := t.TempDir()

	// Create original file
	originalPath := filepath.Join(srcDir, "aaa.txt") // Name starts with 'a' to be first alphabetically
	content := []byte("test content")
	if err := os.WriteFile(originalPath, content, 0644); err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	// Create hardlink (will be second alphabetically)
	hardlinkPath := filepath.Join(srcDir, "zzz.txt") // Name starts with 'z' to be last alphabetically
	if err := os.Link(originalPath, hardlinkPath); err != nil {
		t.Fatalf("Failed to create hardlink: %v", err)
	}

	// Create archive
	archivePath := filepath.Join(t.TempDir(), "test.tar")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	ctx := context.Background()
	files, err := FilesFromDisk(ctx, nil, map[string]string{srcDir: ""})
	if err != nil {
		t.Fatalf("FilesFromDisk failed: %v", err)
	}

	format := Tar{}
	if err := format.Archive(ctx, archiveFile, files); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}
	archiveFile.Close()

	// Extract and verify
	archiveFile, err = os.Open(archivePath)
	if err != nil {
		t.Fatalf("Failed to open archive: %v", err)
	}
	defer archiveFile.Close()

	var foundOriginal, foundHardlink bool
	err = format.Extract(ctx, archiveFile, func(ctx context.Context, file FileInfo) error {
		t.Logf("Extracted: Name=%s, LinkTarget=%q", file.Name(), file.LinkTarget)
		if hdr, ok := file.Header.(*tar.Header); ok {
			t.Logf("  Header: Typeflag=%d (%c), Linkname=%s", hdr.Typeflag, hdr.Typeflag, hdr.Linkname)
		}

		// The first file alphabetically (aaa.txt) should be the regular file
		if file.Name() == "aaa.txt" {
			foundOriginal = true
			if file.LinkTarget != "" {
				t.Errorf("First occurrence should not have LinkTarget, got %q", file.LinkTarget)
			}
			if hdr, ok := file.Header.(*tar.Header); ok {
				if hdr.Typeflag != tar.TypeReg {
					t.Errorf("First occurrence should be TypeReg, got %d", hdr.Typeflag)
				}
			}
		}

		// The second file alphabetically (zzz.txt) should be the hardlink
		if file.Name() == "zzz.txt" {
			foundHardlink = true
			// Verify LinkTarget is set
			if file.LinkTarget == "" {
				t.Error("LinkTarget not set for hardlink during extraction")
			} else {
				t.Logf("Hardlink LinkTarget: %s", file.LinkTarget)
			}
			// For hardlinks, the Typeflag in the header will be TypeLink
			if hdr, ok := file.Header.(*tar.Header); ok {
				if hdr.Typeflag != tar.TypeLink && hdr.Typeflag != tar.TypeReg+1 { // TypeLink or old format hardlink
					t.Errorf("Expected hardlink type, got %d (%c)", hdr.Typeflag, hdr.Typeflag)
				}
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if !foundOriginal {
		t.Error("Original file was not found during extraction")
	}
	if !foundHardlink {
		t.Error("Hardlink was not found during extraction")
	}
}

// TestGnuHardlinksExtraction tests extraction of the gnu-hardlinks.tar test
// archive from the original archiver project (PR #171). This verifies that
// hardlinks are correctly preserved during extraction.
func TestGnuHardlinksExtraction(t *testing.T) {
	archivePath := "testdata/gnu-hardlinks.tar"

	// Open the archive
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("Failed to open archive: %v", err)
	}
	defer archiveFile.Close()

	ctx := context.Background()
	format := Tar{}

	var fileA, fileB *FileInfo
	err = format.Extract(ctx, archiveFile, func(ctx context.Context, file FileInfo) error {
		t.Logf("Extracted: Name=%s, LinkTarget=%q", file.Name(), file.LinkTarget)
		if hdr, ok := file.Header.(*tar.Header); ok {
			t.Logf("  Typeflag=%d (%c)", hdr.Typeflag, hdr.Typeflag)
		}

		// Collect file info
		if file.NameInArchive == "dir-1/dir-2/file-a" {
			fileCopy := file
			fileA = &fileCopy
		} else if file.NameInArchive == "dir-1/dir-2/file-b" {
			fileCopy := file
			fileB = &fileCopy
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify both files were found
	if fileA == nil {
		t.Fatal("file-a not found in archive")
	}
	if fileB == nil {
		t.Fatal("file-b not found in archive")
	}

	// file-a should be a regular file with no LinkTarget
	if fileA.LinkTarget != "" {
		t.Errorf("file-a should be regular file, but has LinkTarget=%q", fileA.LinkTarget)
	}

	// file-b should be a hardlink pointing to file-a
	if fileB.LinkTarget == "" {
		t.Error("file-b should have LinkTarget set (is a hardlink)")
	}
	if fileB.LinkTarget != "dir-1/dir-2/file-a" {
		t.Errorf("file-b LinkTarget = %q, want %q", fileB.LinkTarget, "dir-1/dir-2/file-a")
	}

	// Verify file-b has hardlink type flag
	if hdr, ok := fileB.Header.(*tar.Header); ok {
		// TypeLink='1' or old GNU format
		if hdr.Typeflag != tar.TypeLink && hdr.Typeflag != '1' {
			t.Errorf("file-b Typeflag = %d, expected hardlink type", hdr.Typeflag)
		}
	}
}
