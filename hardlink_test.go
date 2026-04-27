package archives

import (
	"archive/tar"
	"context"
	"os"
	"testing"
)

// TestGnuHardlinksExtraction tests extraction of the gnu-hardlinks.tar test
// archive from the original archiver project (PR #171). This verifies that
// hardlinks are correctly preserved during extraction.
func TestGnuHardlinksExtraction(t *testing.T) {
	archivePath := "testdata/gnu-hardlinks.tar"

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

	if fileA == nil {
		t.Fatal("file-a not found in archive")
	}
	if fileB == nil {
		t.Fatal("file-b not found in archive")
	}

	if fileA.LinkTarget != "" {
		t.Errorf("file-a should be regular file, but has LinkTarget=%q", fileA.LinkTarget)
	}

	if fileB.LinkTarget == "" {
		t.Error("file-b should have LinkTarget set (is a hardlink)")
	}
	if fileB.LinkTarget != "dir-1/dir-2/file-a" {
		t.Errorf("file-b LinkTarget = %q, want %q", fileB.LinkTarget, "dir-1/dir-2/file-a")
	}

	if hdr, ok := fileB.Header.(*tar.Header); ok {
		if hdr.Typeflag != tar.TypeLink && hdr.Typeflag != '1' {
			t.Errorf("file-b Typeflag = %d, expected hardlink type", hdr.Typeflag)
		}
	}
}
