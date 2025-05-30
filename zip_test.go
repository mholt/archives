package archives

import (
	"context"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestZip_ExtractZipWithSymlinks(t *testing.T) {
	zipFile, err := os.Open("testdata/symlinks.zip")
	if err != nil {
		t.Errorf("failed to open zip file: %v", err)
	}
	defer zipFile.Close()

	zip := Zip{}
	extractedFiles := []string{}
	zip.Extract(context.Background(), zipFile, func(ctx context.Context, file FileInfo) error {
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
