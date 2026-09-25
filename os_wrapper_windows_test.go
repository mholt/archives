package archives

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
)

// TestFilesFromDiskTrailingDotOrSpace verifies that files whose names end
// with a dot or a space can be gathered from disk and archived on Windows,
// where the Win32 API strips such trailing characters from ordinary paths.
func TestFilesFromDiskTrailingDotOrSpace(t *testing.T) {
	dir := t.TempDir()

	contents := map[string]string{
		"dot.":   "ends with a dot",
		"space ": "ends with a space",
	}
	for name, data := range contents {
		filename := filepath.Join(dir, name)
		if err := os.WriteFile(fixOSPath(filename), []byte(data), 0o644); err != nil {
			t.Fatalf("creating %q: %v", name, err)
		}
		// os.RemoveAll in the t.TempDir cleanup cannot delete these names
		// without the \\?\ prefix; cleanups run in reverse order, so this
		// runs first.
		t.Cleanup(func() { _ = os.Remove(fixOSPath(filename)) })
	}

	// make sure the names on disk really end with a dot or space
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, ok := contents[e.Name()]; !ok {
			t.Fatalf("unexpected file on disk %q", e.Name())
		}
	}

	all, err := FilesFromDisk(context.Background(), nil, map[string]string{dir: ""})
	if err != nil {
		t.Fatal(err)
	}
	var files []FileInfo
	for _, f := range all {
		if !f.IsDir() {
			files = append(files, f)
		}
	}
	if len(files) != len(contents) {
		t.Fatalf("got %d files, want %d", len(files), len(contents))
	}
	for _, f := range files {
		want, ok := contents[path.Base(f.NameInArchive)]
		if !ok {
			t.Fatalf("unexpected name in archive %q", f.NameInArchive)
		}
		r, err := f.Open()
		if err != nil {
			t.Fatalf("opening %q: %v", f.NameInArchive, err)
		}
		got, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatalf("reading %q: %v", f.NameInArchive, err)
		}
		if string(got) != want {
			t.Fatalf("%q: got %q, want %q", f.NameInArchive, got, want)
		}
	}

	var buf bytes.Buffer
	if err := (Zip{}).Archive(context.Background(), &buf, files); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != len(contents) {
		t.Fatalf("zip has %d entries, want %d", len(zr.File), len(contents))
	}
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if want := contents[path.Base(zf.Name)]; string(got) != want {
			t.Fatalf("zip entry %q: got %q, want %q", zf.Name, got, want)
		}
	}
}
