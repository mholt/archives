package archives_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mholt/archives"
)

func TestZip_ExtractZipWithSymlinks(t *testing.T) {
	zipFile, err := os.Open("testdata/symlinks.zip")
	if err != nil {
		t.Errorf("failed to open zip file: %v", err)
	}
	defer zipFile.Close()

	zip := archives.Zip{}
	extractedFiles := []string{}
	zip.Extract(context.Background(), zipFile, func(ctx context.Context, file archives.FileInfo) error {
		extractedFiles = append(extractedFiles, file.Name())
		if file.Name() == "symlinked" {
			if file.LinkTarget != "../a/hello" {
				t.Errorf("expected symlink target to be '../a/hello', got %s", file.LinkTarget)
			}
		}
		return nil
	})

	if len(extractedFiles) != 5 {
		t.Errorf("expected 5 files to be extracted, got %d", len(extractedFiles))
	}
	sort.Strings(extractedFiles)
	expectedFiles := []string{"a", "b", "hello", "symlinked", "zip_test"}
	if !reflect.DeepEqual(extractedFiles, expectedFiles) {
		t.Errorf("expected files to be %v, got %v", expectedFiles, extractedFiles)
	}
}

type symlinkTestCase struct {
	name           string
	followSymlinks bool
	expectSymlinks bool
}

func TestZip_ArchiveZipWithSymlinks(t *testing.T) {
	testCases := []symlinkTestCase{
		{
			name:           "preserve symlinks",
			followSymlinks: false,
			expectSymlinks: true,
		},
		{
			name:           "follow symlinks",
			followSymlinks: true,
			expectSymlinks: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testSymlinkArchiving(t, tc)
		})
	}
}

func testSymlinkArchiving(t *testing.T, tc symlinkTestCase) {
	testDir := setupTestDir(t)
	archivePath := filepath.Join(testDir.tempDir, "test_with_symlinks.zip")

	ctx := context.Background()
	files, err := archives.FilesFromDisk(
		ctx,
		&archives.FromDiskOptions{FollowSymlinks: tc.followSymlinks},
		testDir.sources,
	)
	if err != nil {
		t.Fatalf("failed to get files: %v", err)
	}

	archive := createAndArchive(t, archivePath, files)
	defer archive.Close()

	extractDir := extractArchive(t, archive, archivePath)
	verifyExtractedContent(t, extractDir, tc.expectSymlinks)
}

type testDirectorySetup struct {
	tempDir         string
	file1Path       string
	file2Path       string
	subDir          string
	file3Path       string
	symlinkToFile   string
	symlinkToDir    string
	relativeSymlink string
	sources         map[string]string
}

func setupTestDir(t *testing.T) *testDirectorySetup {
	tempDir := t.TempDir()

	setup := &testDirectorySetup{
		tempDir:       tempDir,
		file1Path:     filepath.Join(tempDir, "file1.txt"),
		file2Path:     filepath.Join(tempDir, "file2.txt"),
		subDir:        filepath.Join(tempDir, "subdir"),
		symlinkToFile: filepath.Join(tempDir, "symlink_to_file.txt"),
		symlinkToDir:  filepath.Join(tempDir, "symlink_to_dir"),
	}
	setup.file3Path = filepath.Join(setup.subDir, "file3.txt")
	setup.relativeSymlink = filepath.Join(setup.subDir, "relative_symlink.txt")

	createFile(t, setup.file1Path, "content of file 1")
	createFile(t, setup.file2Path, "content of file 2")
	createDir(t, setup.subDir)
	createFile(t, setup.file3Path, "content of file 3")
	createSymlink(t, "file1.txt", setup.symlinkToFile)
	createSymlink(t, "subdir", setup.symlinkToDir)
	createSymlink(t, "../file2.txt", setup.relativeSymlink)

	setup.sources = map[string]string{
		setup.file1Path:       "",
		setup.file2Path:       "",
		setup.subDir:          "",
		setup.symlinkToFile:   "",
		setup.symlinkToDir:    "",
		setup.relativeSymlink: "",
	}

	return setup
}

func createFile(t *testing.T, path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func createDir(t *testing.T, path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

func createSymlink(t *testing.T, target, linkPath string) {
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("failed to create symlink %s -> %s: %v", linkPath, target, err)
	}
}

func createAndArchive(t *testing.T, archivePath string, files []archives.FileInfo) *os.File {
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}

	zip := archives.Zip{}
	ctx := context.Background()
	if err := zip.Archive(ctx, archive, files); err != nil {
		t.Fatalf("failed to archive files: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("failed to close archive: %v", err)
	}

	archive, err = os.Open(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	return archive
}

func extractArchive(t *testing.T, archive *os.File, archivePath string) string {
	extractDir := filepath.Join(filepath.Dir(archivePath), "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("failed to create extract directory: %v", err)
	}

	zip := archives.Zip{}
	ctx := context.Background()
	err := zip.Extract(ctx, archive, func(ctx context.Context, file archives.FileInfo) error {
		if file.IsDir() {
			return os.MkdirAll(filepath.Join(extractDir, file.NameInArchive), file.Mode())
		}

		os.MkdirAll(filepath.Dir(filepath.Join(extractDir, file.NameInArchive)), 0755)
		if file.Mode()&os.ModeSymlink != 0 {
			if file.LinkTarget == "" {
				return fmt.Errorf("symlink target is empty")
			}
			return os.Symlink(file.LinkTarget, filepath.Join(extractDir, file.NameInArchive))
		}

		handle, err := file.Open()
		if err != nil {
			return err
		}
		defer handle.Close()
		dest, err := os.Create(filepath.Join(extractDir, file.NameInArchive))
		if err != nil {
			return err
		}
		defer dest.Close()
		_, err = io.Copy(dest, handle)
		return err
	})
	if err != nil {
		t.Fatalf("failed to extract archive: %v", err)
	}
	return extractDir
}

func verifyExtractedContent(t *testing.T, extractDir string, expectSymlinks bool) {
	verifyFileContent(t, extractDir, "file1.txt", "content of file 1")
	verifyFileContent(t, extractDir, "file2.txt", "content of file 2")
	verifyFileContent(t, extractDir, "subdir/file3.txt", "content of file 3")

	if expectSymlinks {
		verifySymlink(t, extractDir, "symlink_to_file.txt", "file1.txt")
		verifySymlink(t, extractDir, "symlink_to_dir", "subdir")
		verifySymlink(t, extractDir, "relative_symlink.txt", "../file2.txt")
	} else {
		verifyFileContent(t, extractDir, "symlink_to_file.txt", "content of file 1")
		verifyFileContent(t, extractDir, "relative_symlink.txt", "content of file 2")
		verifyIsDirectory(t, extractDir, "symlink_to_dir")
	}
}

func verifyFileContent(t *testing.T, baseDir, relativePath, expectedContent string) {
	filePath := filepath.Join(baseDir, relativePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Errorf("failed to read %s: %v", relativePath, err)
		return
	}
	if string(content) != expectedContent {
		t.Errorf("expected content %q in %s, got %q", expectedContent, relativePath, string(content))
	}
}

func verifySymlink(t *testing.T, baseDir, relativePath, expectedTarget string) {
	filePath := filepath.Join(baseDir, relativePath)
	stat, err := os.Lstat(filePath)
	if err != nil {
		t.Errorf("failed to lstat %s: %v", relativePath, err)
		return
	}
	if stat.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink, got mode %s", relativePath, stat.Mode())
		return
	}
	target, err := os.Readlink(filePath)
	if err != nil {
		t.Errorf("failed to read symlink %s: %v", relativePath, err)
		return
	}
	if target != expectedTarget {
		t.Errorf("expected symlink %s to point to %s, got %s", relativePath, expectedTarget, target)
	}
}

func verifyIsDirectory(t *testing.T, baseDir, relativePath string) {
	filePath := filepath.Join(baseDir, relativePath)
	stat, err := os.Lstat(filePath)
	if err != nil {
		t.Errorf("failed to lstat %s: %v", relativePath, err)
		return
	}
	if !stat.IsDir() {
		t.Errorf("expected %s to be a directory, got mode %s", relativePath, stat.Mode())
	}
}
