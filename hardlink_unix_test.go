//go:build !windows

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
	tmpDir := t.TempDir()

	originalPath := filepath.Join(tmpDir, "original.txt")
	content := []byte("test content for hardlink")
	if err := os.WriteFile(originalPath, content, 0644); err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	hardlinkPath := filepath.Join(tmpDir, "hardlink.txt")
	if err := os.Link(originalPath, hardlinkPath); err != nil {
		t.Fatalf("Failed to create hardlink: %v", err)
	}

	origInfo, _ := os.Stat(originalPath)
	linkInfo, _ := os.Stat(hardlinkPath)
	origStat := origInfo.Sys().(*syscall.Stat_t)
	linkStat := linkInfo.Sys().(*syscall.Stat_t)

	if origStat.Ino != linkStat.Ino {
		t.Fatalf("Files are not hardlinked: orig inode=%d, link inode=%d", origStat.Ino, linkStat.Ino)
	}

	ctx := context.Background()
	files, err := FilesFromDisk(ctx, nil, map[string]string{tmpDir + string(filepath.Separator): ""})
	if err != nil {
		t.Fatalf("FilesFromDisk failed: %v", err)
	}

	if len(files) != 2 {
		for i, f := range files {
			t.Logf("File %d: Name=%s, NameInArchive=%s, LinkTarget=%q, IsRegular=%v",
				i, f.Name(), f.NameInArchive, f.LinkTarget, f.Mode().IsRegular())
		}
		t.Fatalf("Expected 2 files, got %d", len(files))
	}

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

	if hardlinkEntry.LinkTarget != originalEntry.NameInArchive {
		t.Errorf("LinkTarget = %q, want %q", hardlinkEntry.LinkTarget, originalEntry.NameInArchive)
	}
}

// TestHardlinkInTarArchive verifies that hardlinks are correctly written
// to tar archives with TypeLink headers.
func TestHardlinkInTarArchive(t *testing.T) {
	tmpDir := t.TempDir()

	originalPath := filepath.Join(tmpDir, "file1.txt")
	content := []byte("shared content")
	if err := os.WriteFile(originalPath, content, 0644); err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	link2Path := filepath.Join(tmpDir, "file2.txt")
	link3Path := filepath.Join(tmpDir, "file3.txt")
	if err := os.Link(originalPath, link2Path); err != nil {
		t.Fatalf("Failed to create hardlink 2: %v", err)
	}
	if err := os.Link(originalPath, link3Path); err != nil {
		t.Fatalf("Failed to create hardlink 3: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "test.tar")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}
	defer archiveFile.Close()

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
			if hdr.Name != "testdir/file1.txt" {
				t.Errorf("Unexpected regular file: %s", hdr.Name)
			}
			if hdr.Size == 0 {
				t.Error("Regular file has zero size")
			}
		case tar.TypeLink:
			hardlinks++
			if hdr.Name != "testdir/file2.txt" && hdr.Name != "testdir/file3.txt" {
				t.Errorf("Unexpected hardlink: %s", hdr.Name)
			}
			if hdr.Size != 0 {
				t.Errorf("Hardlink %s has non-zero size: %d", hdr.Name, hdr.Size)
			}
			if hdr.Linkname != "testdir/file1.txt" {
				t.Errorf("Hardlink %s points to %s, want testdir/file1.txt", hdr.Name, hdr.Linkname)
			}
		}
	}

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
	srcDir := t.TempDir()

	originalPath := filepath.Join(srcDir, "aaa.txt")
	content := []byte("test content")
	if err := os.WriteFile(originalPath, content, 0644); err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	hardlinkPath := filepath.Join(srcDir, "zzz.txt")
	if err := os.Link(originalPath, hardlinkPath); err != nil {
		t.Fatalf("Failed to create hardlink: %v", err)
	}

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

		if file.Name() == "zzz.txt" {
			foundHardlink = true
			if file.LinkTarget == "" {
				t.Error("LinkTarget not set for hardlink during extraction")
			} else {
				t.Logf("Hardlink LinkTarget: %s", file.LinkTarget)
			}
			if hdr, ok := file.Header.(*tar.Header); ok {
				if hdr.Typeflag != tar.TypeLink && hdr.Typeflag != tar.TypeReg+1 {
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
