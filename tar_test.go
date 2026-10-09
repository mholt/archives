package archives

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"testing"
)

func createTestTarArchive(t *testing.T, hdr *tar.Header, body []byte) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader failed: %v", err)
	}
	if len(body) > 0 {
		if _, err := tw.Write(body); err != nil {
			t.Fatalf("Write body failed: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close tar writer failed: %v", err)
	}
	return buf.Bytes()
}

func TestTarExtract_DefaultPreservesOwnership(t *testing.T) {
	ctx := context.Background()
	archiveData := createTestTarArchive(t, &tar.Header{
		Name:     "test.txt",
		Mode:     0644,
		Size:     11,
		Typeflag: tar.TypeReg,
		Uid:      1234,
		Gid:      5678,
		Uname:    "testuser",
		Gname:    "testgroup",
	}, []byte("hello world"))

	tarFormat := Tar{}
	var extractedFiles int

	err := tarFormat.Extract(ctx, bytes.NewReader(archiveData), func(ctx context.Context, file FileInfo) error {
		extractedFiles++
		hdr, ok := file.Header.(*tar.Header)
		if !ok {
			t.Fatalf("expected *tar.Header, got %T", file.Header)
		}
		if hdr.Uid != 1234 {
			t.Errorf("expected Uid 1234, got %d", hdr.Uid)
		}
		if hdr.Gid != 5678 {
			t.Errorf("expected Gid 5678, got %d", hdr.Gid)
		}
		if hdr.Uname != "testuser" {
			t.Errorf("expected Uname 'testuser', got %q", hdr.Uname)
		}
		if hdr.Gname != "testgroup" {
			t.Errorf("expected Gname 'testgroup', got %q", hdr.Gname)
		}

		sysHdr, ok := file.Sys().(*tar.Header)
		if !ok {
			t.Fatalf("expected sys to be *tar.Header, got %T", file.Sys())
		}
		if sysHdr.Uid != 1234 || sysHdr.Gid != 5678 {
			t.Errorf("expected sys Uid/Gid to match header, got %d/%d", sysHdr.Uid, sysHdr.Gid)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if extractedFiles != 1 {
		t.Fatalf("expected 1 extracted file, got %d", extractedFiles)
	}
}

func TestTarExtract_SkipUIDGID(t *testing.T) {
	ctx := context.Background()
	archiveData := createTestTarArchive(t, &tar.Header{
		Name:     "data.bin",
		Mode:     0644,
		Size:     4,
		Typeflag: tar.TypeReg,
		Uid:      1000,
		Gid:      1000,
		Uname:    "developer",
		Gname:    "developers",
		PAXRecords: map[string]string{
			"uid":   "1000",
			"gid":   "1000",
			"uname": "developer",
			"gname": "developers",
		},
	}, []byte("data"))

	tarFormat := Tar{SkipUIDGID: true}
	var extractedFiles int

	err := tarFormat.Extract(ctx, bytes.NewReader(archiveData), func(ctx context.Context, file FileInfo) error {
		extractedFiles++
		hdr, ok := file.Header.(*tar.Header)
		if !ok {
			t.Fatalf("expected *tar.Header, got %T", file.Header)
		}
		if hdr.Uid != 0 {
			t.Errorf("expected Uid 0, got %d", hdr.Uid)
		}
		if hdr.Gid != 0 {
			t.Errorf("expected Gid 0, got %d", hdr.Gid)
		}
		if hdr.Uname != "" {
			t.Errorf("expected empty Uname, got %q", hdr.Uname)
		}
		if hdr.Gname != "" {
			t.Errorf("expected empty Gname, got %q", hdr.Gname)
		}
		if _, exists := hdr.PAXRecords["uid"]; exists {
			t.Errorf("PAXRecords['uid'] should be removed")
		}
		if _, exists := hdr.PAXRecords["gid"]; exists {
			t.Errorf("PAXRecords['gid'] should be removed")
		}
		if _, exists := hdr.PAXRecords["uname"]; exists {
			t.Errorf("PAXRecords['uname'] should be removed")
		}
		if _, exists := hdr.PAXRecords["gname"]; exists {
			t.Errorf("PAXRecords['gname'] should be removed")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if extractedFiles != 1 {
		t.Fatalf("expected 1 extracted file, got %d", extractedFiles)
	}
}

func TestTarExtract_NumericUIDGID(t *testing.T) {
	ctx := context.Background()
	archiveData := createTestTarArchive(t, &tar.Header{
		Name:     "numeric.txt",
		Mode:     0644,
		Size:     5,
		Typeflag: tar.TypeReg,
		Uid:      2001,
		Gid:      2002,
		Uname:    "someuser",
		Gname:    "somegroup",
		PAXRecords: map[string]string{
			"uname": "someuser",
			"gname": "somegroup",
		},
	}, []byte("test\n"))

	tarFormat := Tar{NumericUIDGID: true}
	var extractedFiles int

	err := tarFormat.Extract(ctx, bytes.NewReader(archiveData), func(ctx context.Context, file FileInfo) error {
		extractedFiles++
		hdr, ok := file.Header.(*tar.Header)
		if !ok {
			t.Fatalf("expected *tar.Header, got %T", file.Header)
		}
		if hdr.Uid != 2001 {
			t.Errorf("expected Uid 2001, got %d", hdr.Uid)
		}
		if hdr.Gid != 2002 {
			t.Errorf("expected Gid 2002, got %d", hdr.Gid)
		}
		if hdr.Uname != "" {
			t.Errorf("expected empty Uname, got %q", hdr.Uname)
		}
		if hdr.Gname != "" {
			t.Errorf("expected empty Gname, got %q", hdr.Gname)
		}
		if _, exists := hdr.PAXRecords["uname"]; exists {
			t.Errorf("PAXRecords['uname'] should be removed")
		}
		if _, exists := hdr.PAXRecords["gname"]; exists {
			t.Errorf("PAXRecords['gname'] should be removed")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if extractedFiles != 1 {
		t.Fatalf("expected 1 extracted file, got %d", extractedFiles)
	}
}

func TestTarExtract_CustomOwnership(t *testing.T) {
	ctx := context.Background()
	archiveData := createTestTarArchive(t, &tar.Header{
		Name:     "custom.txt",
		Mode:     0644,
		Size:     3,
		Typeflag: tar.TypeReg,
		Uid:      100,
		Gid:      200,
		Uname:    "originaluser",
		Gname:    "originalgroup",
	}, []byte("abc"))

	tarFormat := Tar{
		Uid:   501,
		Gid:   502,
		Uname: "customuser",
		Gname: "customgroup",
	}

	err := tarFormat.Extract(ctx, bytes.NewReader(archiveData), func(ctx context.Context, file FileInfo) error {
		hdr := file.Header.(*tar.Header)
		if hdr.Uid != 501 {
			t.Errorf("expected Uid 501, got %d", hdr.Uid)
		}
		if hdr.Gid != 502 {
			t.Errorf("expected Gid 502, got %d", hdr.Gid)
		}
		if hdr.Uname != "customuser" {
			t.Errorf("expected Uname 'customuser', got %q", hdr.Uname)
		}
		if hdr.Gname != "customgroup" {
			t.Errorf("expected Gname 'customgroup', got %q", hdr.Gname)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
}

func TestTarExtract_SkipUIDGIDWithOverride(t *testing.T) {
	ctx := context.Background()
	archiveData := createTestTarArchive(t, &tar.Header{
		Name:     "override.txt",
		Mode:     0644,
		Size:     3,
		Typeflag: tar.TypeReg,
		Uid:      100,
		Gid:      200,
		Uname:    "origuser",
		Gname:    "origgroup",
	}, []byte("123"))

	tarFormat := Tar{
		SkipUIDGID: true,
		Uid:        999,
	}

	err := tarFormat.Extract(ctx, bytes.NewReader(archiveData), func(ctx context.Context, file FileInfo) error {
		hdr := file.Header.(*tar.Header)
		if hdr.Uid != 999 {
			t.Errorf("expected Uid 999, got %d", hdr.Uid)
		}
		if hdr.Gid != 0 {
			t.Errorf("expected Gid 0, got %d", hdr.Gid)
		}
		if hdr.Uname != "" {
			t.Errorf("expected empty Uname, got %q", hdr.Uname)
		}
		if hdr.Gname != "" {
			t.Errorf("expected empty Gname, got %q", hdr.Gname)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
}

func TestTarArchive_SkipUIDGID(t *testing.T) {
	ctx := context.Background()
	outBuf := new(bytes.Buffer)

	dummyHdr := &tar.Header{
		Name:     "archive_file.txt",
		Mode:     0644,
		Size:     4,
		Typeflag: tar.TypeReg,
		Uid:      1001,
		Gid:      1002,
		Uname:    "builder",
		Gname:    "builders",
	}

	file := FileInfo{
		FileInfo:      dummyHdr.FileInfo(),
		Header:        dummyHdr,
		NameInArchive: "archive_file.txt",
		Open: func() (fs.File, error) {
			return fileInArchive{
				ReadCloser: io.NopCloser(bytes.NewReader([]byte("test"))),
				info:       dummyHdr.FileInfo(),
			}, nil
		},
	}

	tarFormat := Tar{SkipUIDGID: true}
	err := tarFormat.Archive(ctx, outBuf, []FileInfo{file})
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	// Verify archived header
	tr := tar.NewReader(outBuf)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("Reading archived header failed: %v", err)
	}
	if hdr.Uid != 0 {
		t.Errorf("expected Uid 0 in archive, got %d", hdr.Uid)
	}
	if hdr.Gid != 0 {
		t.Errorf("expected Gid 0 in archive, got %d", hdr.Gid)
	}
	if hdr.Uname != "" {
		t.Errorf("expected empty Uname in archive, got %q", hdr.Uname)
	}
	if hdr.Gname != "" {
		t.Errorf("expected empty Gname in archive, got %q", hdr.Gname)
	}
}

func TestTarExtract_FixtureGnuHardlinks_SkipUIDGID(t *testing.T) {
	archiveFile, err := os.Open("testdata/gnu-hardlinks.tar")
	if err != nil {
		t.Fatalf("Failed to open fixture: %v", err)
	}
	defer archiveFile.Close()

	ctx := context.Background()
	tarFormat := Tar{SkipUIDGID: true}
	var count int

	err = tarFormat.Extract(ctx, archiveFile, func(ctx context.Context, file FileInfo) error {
		count++
		hdr, ok := file.Header.(*tar.Header)
		if !ok {
			t.Fatalf("expected *tar.Header, got %T", file.Header)
		}
		if hdr.Uid != 0 {
			t.Errorf("file %s: expected Uid 0, got %d", hdr.Name, hdr.Uid)
		}
		if hdr.Gid != 0 {
			t.Errorf("file %s: expected Gid 0, got %d", hdr.Name, hdr.Gid)
		}
		if hdr.Uname != "" {
			t.Errorf("file %s: expected empty Uname, got %q", hdr.Name, hdr.Uname)
		}
		if hdr.Gname != "" {
			t.Errorf("file %s: expected empty Gname, got %q", hdr.Name, hdr.Gname)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if count == 0 {
		t.Fatal("expected files to be extracted from fixture")
	}
}
