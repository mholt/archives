package archives

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"reflect"
	"testing"
	"time"
)

func tarExtractTestArchive(t *testing.T, headers ...*tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, hdr := range headers {
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(make([]byte, hdr.Size)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTarExtractHeaderError(t *testing.T) {
	// Keep the regression bounded on versions which repeatedly log a sticky error.
	output := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(output) })
	valid := tarExtractTestArchive(t, &tar.Header{Name: "before", Size: 512})
	badHeader := bytes.Repeat([]byte("x"), 512)
	tests := []struct {
		name string
		data []byte
		want error
		seen int
	}{
		{"short header", []byte("not-a-tar"), io.ErrUnexpectedEOF, 0},
		{"invalid header", badHeader, tar.ErrHeader, 0},
		{"invalid header after file", append(bytes.Clone(valid[:1024]), badHeader...), tar.ErrHeader, 1},
		{"truncated file", valid[:513], io.ErrUnexpectedEOF, 1},
	}
	for _, tc := range tests {
		for _, keepGoing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/continue=%v", tc.name, keepGoing), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				var seen int
				err := (Tar{ContinueOnError: keepGoing}).Extract(ctx, bytes.NewReader(tc.data), func(context.Context, FileInfo) error {
					seen++
					return nil
				})
				if !errors.Is(err, tc.want) {
					t.Errorf("error = %v, want %v", err, tc.want)
				}
				if ctx.Err() != nil {
					t.Errorf("extraction waited for context cancellation: %v", ctx.Err())
				}
				if seen != tc.seen {
					t.Errorf("handled %d files, want %d", seen, tc.seen)
				}
			})
		}
	}
}

func TestTarExtractContinueOnErrorInsecurePath(t *testing.T) {
	t.Setenv("GODEBUG", "tarinsecurepath=0")
	data := tarExtractTestArchive(t, &tar.Header{Name: "../outside", Size: 1}, &tar.Header{Name: "inside", Size: 1})
	for _, keepGoing := range []bool{false, true} {
		t.Run(fmt.Sprintf("continue=%v", keepGoing), func(t *testing.T) {
			var names []string
			err := (Tar{ContinueOnError: keepGoing}).Extract(context.Background(), bytes.NewReader(data), func(_ context.Context, file FileInfo) error {
				names = append(names, file.NameInArchive)
				return nil
			})
			if keepGoing {
				if err != nil || !reflect.DeepEqual(names, []string{"inside"}) {
					t.Fatalf("names = %v, error = %v", names, err)
				}
			} else if !errors.Is(err, tar.ErrInsecurePath) || len(names) != 0 {
				t.Fatalf("names = %v, error = %v; want ErrInsecurePath", names, err)
			}
		})
	}
}

func TestTarExtractContinueOnErrorHandler(t *testing.T) {
	data := tarExtractTestArchive(t, &tar.Header{Name: "dir/", Typeflag: tar.TypeDir}, &tar.Header{Name: "dir/file"}, &tar.Header{Name: "last"})
	callbackErr := errors.New("callback failed")
	for _, tc := range []struct {
		name      string
		err       error
		wantNames []string
		wantErr   error
	}{
		{"skip directory", fs.SkipDir, []string{"dir/", "last"}, nil},
		{"skip all", fs.SkipAll, []string{"dir/"}, nil},
		{"callback error", callbackErr, []string{"dir/"}, callbackErr},
		{"cancel context", context.Canceled, []string{"dir/"}, context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var names []string
			err := (Tar{ContinueOnError: true}).Extract(ctx, bytes.NewReader(data), func(_ context.Context, file FileInfo) error {
				names = append(names, file.NameInArchive)
				if file.NameInArchive == "dir/" {
					if tc.err == context.Canceled {
						cancel()
						return nil
					}
					return tc.err
				}
				return nil
			})
			if !errors.Is(err, tc.wantErr) || !reflect.DeepEqual(names, tc.wantNames) {
				t.Fatalf("names = %v, error = %v; want %v, %v", names, err, tc.wantNames, tc.wantErr)
			}
		})
	}
}
