package archives

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestPathWithoutTopDir(t *testing.T) {
	for i, tc := range []struct {
		input, expect string
	}{
		{
			input:  "a/b/c",
			expect: "b/c",
		},
		{
			input:  "b/c",
			expect: "c",
		},
		{
			input:  "c",
			expect: "c",
		},
		{
			input:  "",
			expect: "",
		},
	} {
		if actual := pathWithoutTopDir(tc.input); actual != tc.expect {
			t.Errorf("Test %d (input=%s): Expected '%s' but got '%s'", i, tc.input, tc.expect, actual)
		}
	}
}

func TestSplitPath(t *testing.T) {
	d := DeepFS{}
	for i, testCase := range []struct {
		input, expectedReal, expectedInner string
	}{
		{
			input:         "/",
			expectedReal:  "/",
			expectedInner: "",
		},
		{
			input:         "foo",
			expectedReal:  "foo",
			expectedInner: "",
		},
		{
			input:         "foo/bar",
			expectedReal:  filepath.Join("foo", "bar"),
			expectedInner: "",
		},
		{
			input:         "foo.zip",
			expectedReal:  filepath.Join("foo.zip"),
			expectedInner: ".",
		},
		{
			input:         "foo.zip/a",
			expectedReal:  "foo.zip",
			expectedInner: "a",
		},
		{
			input:         "foo.zip/a/b",
			expectedReal:  "foo.zip",
			expectedInner: "a/b",
		},
		{
			input:         "a/b/foobar.zip/c",
			expectedReal:  filepath.Join("a", "b", "foobar.zip"),
			expectedInner: "c",
		},
		{
			input:         "a/foo.zip/b/test.tar",
			expectedReal:  filepath.Join("a", "foo.zip"),
			expectedInner: "b/test.tar",
		},
		{
			input:         "a/foo.zip/b/test.tar/c",
			expectedReal:  filepath.Join("a", "foo.zip"),
			expectedInner: "b/test.tar/c",
		},
	} {
		actualReal, actualInner := d.SplitPath(testCase.input)
		if actualReal != testCase.expectedReal {
			t.Errorf("Test %d (input=%q): expected real path %q but got %q", i, testCase.input, testCase.expectedReal, actualReal)
		}
		if actualInner != testCase.expectedInner {
			t.Errorf("Test %d (input=%q): expected inner path %q but got %q", i, testCase.input, testCase.expectedInner, actualInner)
		}
	}
}

func TestPathContainsArchive(t *testing.T) {
	for i, testCase := range []struct {
		input    string
		expected bool
	}{
		{
			input:    "",
			expected: false,
		},
		{
			input:    "foo",
			expected: false,
		},
		{
			input:    "foo.zip",
			expected: true,
		},
		{
			input:    "a/b/c.tar.gz",
			expected: true,
		},
		{
			input:    "a/b/c.tar.gz/d",
			expected: true,
		},
		{
			input:    "a/b/c.txt",
			expected: false,
		},
	} {
		actual := PathContainsArchive(testCase.input)
		if actual != testCase.expected {
			t.Errorf("Test %d (input=%q): expected %v but got %v", i, testCase.input, testCase.expected, actual)
		}
	}
}

var (
	//go:embed testdata/test.zip
	testZIP []byte
	//go:embed testdata/unordered.zip
	unorderZip []byte
)

func TestSelfTar(t *testing.T) {
	fn := "testdata/self-tar.tar"
	fh, err := os.Open(fn)
	if err != nil {
		t.Errorf("Could not load test tar: %v", fn)
	}
	fstat, err := os.Stat(fn)
	if err != nil {
		t.Errorf("Could not stat test tar: %v", fn)
	}
	fsys := &ArchiveFS{
		Stream: io.NewSectionReader(fh, 0, fstat.Size()),
		Format: Tar{},
	}
	var count int
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if count > 10 {
			t.Error("walking test tar appears to be recursing in error")
			return fmt.Errorf("recursing tar: %v", fn)
		}
		count++
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}

func ExampleArchiveFS_Stream() {
	fsys := &ArchiveFS{
		Stream: io.NewSectionReader(bytes.NewReader(testZIP), 0, int64(len(testZIP))),
		Format: Zip{},
	}
	// You can serve the contents in a web server:
	http.Handle("/static", http.StripPrefix("/static",
		http.FileServer(http.FS(fsys))))

	// Or read the files using fs functions:
	dis, err := fsys.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}
	for _, di := range dis {
		fmt.Println(di.Name())
		b, err := fs.ReadFile(fsys, path.Join(".", di.Name()))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(bytes.Contains(b, []byte("granted")))
	}
	// Output:
	// LICENSE
	// true
}

func TestArchiveFS_ReadDir(t *testing.T) {
	for _, tc := range []struct {
		name    string
		archive ArchiveFS
		want    map[string][]string
	}{
		{
			name: "test.zip",
			archive: ArchiveFS{
				Stream: io.NewSectionReader(bytes.NewReader(testZIP), 0, int64(len(testZIP))),
				Format: Zip{},
			},
			// unzip -l testdata/test.zip
			want: map[string][]string{
				".": {"LICENSE"},
			},
		},
		{
			name: "unordered.zip",
			archive: ArchiveFS{
				Stream: io.NewSectionReader(bytes.NewReader(unorderZip), 0, int64(len(unorderZip))),
				Format: Zip{},
			},
			// unzip -l testdata/unordered.zip, note entry 1/1 and 1/2 are separated by contents of directory 2
			want: map[string][]string{
				".": {"1", "2"},
				"1": {"1", "2"},
				"2": {"1"},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fsys := tc.archive
			for baseDir, wantLS := range tc.want {
				t.Run(fmt.Sprintf("ReadDir(%q)", baseDir), func(t *testing.T) {
					dis, err := fsys.ReadDir(baseDir)
					if err != nil {
						t.Error(err)
					}

					dirs := []string{}
					for _, di := range dis {
						dirs = append(dirs, di.Name())
					}

					// Stabilize the sort order
					sort.Strings(dirs)

					if !reflect.DeepEqual(wantLS, dirs) {
						t.Errorf("ReadDir() got: %v, want: %v", dirs, wantLS)
					}
				})

				// Uncomment to reproduce https://github.com/mholt/archiver/issues/340.
				t.Run(fmt.Sprintf("Open(%s)", baseDir), func(t *testing.T) {
					f, err := fsys.Open(baseDir)
					if err != nil {
						t.Errorf("fsys.Open(%q): %#v %s", baseDir, err, err)
						return
					}

					rdf, ok := f.(fs.ReadDirFile)
					if !ok {
						t.Errorf("fsys.Open(%q) did not return a fs.ReadDirFile, got: %#v", baseDir, f)
					}

					dis, err := rdf.ReadDir(-1)
					if err != nil {
						t.Error(err)
					}

					dirs := []string{}
					for _, di := range dis {
						dirs = append(dirs, di.Name())
					}

					// Stabilize the sort order
					sort.Strings(dirs)

					if !reflect.DeepEqual(wantLS, dirs) {
						t.Errorf("Open().ReadDir(-1) got: %v, want: %v", dirs, wantLS)
					}
				})
			}
		})
	}
}

func TestFileSystem(t *testing.T) {
	ctx := context.Background()
	filename := "testdata/test.zip"

	checkFS := func(t *testing.T, fsys fs.FS) {
		license, err := fsys.Open("LICENSE")
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(license)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) == 0 {
			t.Fatal("empty file")
		}
		err = license.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Run("filename", func(t *testing.T) {
		fsys, err := FileSystem(ctx, filename, nil)
		if err != nil {
			t.Fatal(err)
		}
		checkFS(t, fsys)
	})

	t.Run("stream", func(t *testing.T) {
		f, err := os.Open(filename)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			err = f.Close()
			if err != nil {
				t.Error(err)
			}
		})
		fsys, err := FileSystem(ctx, "", f)
		if err != nil {
			t.Fatal(err)
		}
		checkFS(t, fsys)
	})

	t.Run("filename and stream", func(t *testing.T) {
		f, err := os.Open(filename)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			err = f.Close()
			if err != nil {
				t.Error(err)
			}
		})
		fsys, err := FileSystem(ctx, "test.zip", f)
		if err != nil {
			t.Fatal(err)
		}
		checkFS(t, fsys)
	})
}
