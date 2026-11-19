package archives_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/mholt/archives"
)

type rejectFinalWrite struct {
	err   error
	calls int
}

func (w *rejectFinalWrite) Write([]byte) (int, error) {
	w.calls++
	return 0, w.err
}

type rejectInsertWrite struct {
	*bytes.Reader
	err   error
	calls int
}

func (w *rejectInsertWrite) Write([]byte) (int, error) {
	w.calls++
	return 0, w.err
}

func TestTarArchiveReportsFinalizationError(t *testing.T) {
	ctx := context.Background()
	var good bytes.Buffer
	if err := (archives.Tar{}).Archive(ctx, &good, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := tar.NewReader(&good).Next(); err != io.EOF {
		t.Fatalf("empty TAR control: got %v, want EOF", err)
	}

	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "Archive", true: "ArchiveAsync"}[async], func(t *testing.T) {
			writeErr := errors.New("final archive write rejected")
			output := &rejectFinalWrite{err: writeErr}
			var err error
			if async {
				jobs := make(chan archives.ArchiveAsyncJob)
				close(jobs)
				err = (archives.Tar{}).ArchiveAsync(ctx, output, jobs)
			} else {
				err = (archives.Tar{}).Archive(ctx, output, nil)
			}
			if output.calls == 0 {
				t.Fatal("finalization never reached the output writer")
			}
			if !errors.Is(err, writeErr) {
				t.Fatalf("archive returned %v after finalization write failed; want %v", err, writeErr)
			}
		})
	}
}

func TestZipArchiveReportsFinalizationError(t *testing.T) {
	ctx := context.Background()
	var good bytes.Buffer
	if err := (archives.Zip{}).Archive(ctx, &good, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := zip.NewReader(bytes.NewReader(good.Bytes()), int64(good.Len())); err != nil {
		t.Fatalf("empty ZIP control: %v", err)
	}

	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "Archive", true: "ArchiveAsync"}[async], func(t *testing.T) {
			writeErr := errors.New("final archive write rejected")
			output := &rejectFinalWrite{err: writeErr}
			var err error
			if async {
				jobs := make(chan archives.ArchiveAsyncJob)
				close(jobs)
				err = (archives.Zip{}).ArchiveAsync(ctx, output, jobs)
			} else {
				err = (archives.Zip{}).Archive(ctx, output, nil)
			}
			if output.calls == 0 {
				t.Fatal("finalization never reached the output writer")
			}
			if !errors.Is(err, writeErr) {
				t.Fatalf("archive returned %v after finalization write failed; want %v", err, writeErr)
			}
		})
	}
}

func TestTarInsertReportsFinalizationError(t *testing.T) {
	var original bytes.Buffer
	if err := (archives.Tar{}).Archive(context.Background(), &original, nil); err != nil {
		t.Fatal(err)
	}
	writeErr := errors.New("insert trailer write rejected")
	stream := &rejectInsertWrite{Reader: bytes.NewReader(original.Bytes()), err: writeErr}
	err := (archives.Tar{}).Insert(context.Background(), stream, nil)
	if stream.calls == 0 {
		t.Fatal("insert finalization never reached the output writer")
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("insert returned %v after finalization write failed; want %v", err, writeErr)
	}
}

func TestZipInsertReportsFinalizationError(t *testing.T) {
	var original bytes.Buffer
	if err := (archives.Zip{}).Archive(context.Background(), &original, nil); err != nil {
		t.Fatal(err)
	}
	writeErr := errors.New("insert directory write rejected")
	stream := &rejectInsertWrite{Reader: bytes.NewReader(original.Bytes()), err: writeErr}
	err := (archives.Zip{}).Insert(context.Background(), stream, nil)
	if stream.calls == 0 {
		t.Fatal("insert finalization never reached the output writer")
	}
	if !errors.Is(err, writeErr) {
		t.Fatalf("insert returned %v after finalization write failed; want %v", err, writeErr)
	}
}
