package archives

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTarInsertBlockBoundary(t *testing.T) {
	tests := []struct {
		name   string
		header *tar.Header
	}{
		{name: "empty archive"},
		{name: "directory", header: &tar.Header{Name: "dir/", Typeflag: tar.TypeDir, Mode: 0755}},
		{name: "PAX long name", header: &tar.Header{Name: strings.Repeat("a", 200), Size: 512, Format: tar.FormatPAX}},
		{name: "GNU long name", header: &tar.Header{Name: strings.Repeat("b", 200), Size: 512, Format: tar.FormatGNU}},
	}
	for _, size := range []int64{0, 1, 511, 512, 513, 1023, 1024} {
		tests = append(tests, struct {
			name   string
			header *tar.Header
		}{fmt.Sprintf("size=%d", size), &tar.Header{Name: "before", Size: size, Mode: 0600}})
	}
	for _, tc := range tests {
		for _, trailerSize := range []int64{0, 1024, 10240} {
			t.Run(fmt.Sprintf("%s/trailer=%d", tc.name, trailerSize), func(t *testing.T) {
				archive, err := os.CreateTemp(t.TempDir(), "*.tar")
				if err != nil {
					t.Fatal(err)
				}
				defer archive.Close()
				tw := tar.NewWriter(archive)
				var before []byte
				if tc.header != nil {
					if err := tw.WriteHeader(tc.header); err != nil {
						t.Fatal(err)
					}
					before = bytes.Repeat([]byte("x"), int(tc.header.Size))
					if _, err := tw.Write(before); err != nil {
						t.Fatal(err)
					}
				}
				if err := tw.Close(); err != nil {
					t.Fatal(err)
				}
				end, err := archive.Seek(0, io.SeekCurrent)
				if err != nil {
					t.Fatal(err)
				}
				if err := archive.Truncate(end - 1024 + trailerSize); err != nil {
					t.Fatal(err)
				}

				// Insert twice, including after an appended block-aligned file.
				for _, name := range []string{"after", "last"} {
					if _, err := archive.Seek(0, io.SeekStart); err != nil {
						t.Fatal(err)
					}
					files := fstest.MapFS{name: &fstest.MapFile{Data: bytes.Repeat([]byte(name[:1]), 512), Mode: 0600}}
					info, err := fs.Stat(files, name)
					if err != nil {
						t.Fatal(err)
					}
					file := FileInfo{FileInfo: info, NameInArchive: name, Open: func() (fs.File, error) { return files.Open(name) }}
					if err := (Tar{}).Insert(context.Background(), archive, []FileInfo{file}); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := archive.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				tr := tar.NewReader(archive)
				check := func(name string, data []byte) {
					t.Helper()
					hdr, err := tr.Next()
					if err != nil {
						t.Fatalf("reading %s: %v", name, err)
					}
					if hdr.Name != name {
						t.Fatalf("name = %q, want %q", hdr.Name, name)
					}
					got, err := io.ReadAll(tr)
					if err != nil || !bytes.Equal(got, data) {
						t.Fatalf("content of %s: got %d bytes, want %d; error: %v", name, len(got), len(data), err)
					}
				}
				if tc.header != nil {
					check(tc.header.Name, before)
				}
				check("after", bytes.Repeat([]byte("a"), 512))
				check("last", bytes.Repeat([]byte("l"), 512))
				if _, err := tr.Next(); err != io.EOF {
					t.Fatalf("end of archive: %v", err)
				}
			})
		}
	}
}
