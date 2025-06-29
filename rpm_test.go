package archives

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRpm_Match(t *testing.T) {
	ctx := context.Background()
	t.Run("By extension", func(t *testing.T) {
		res, err := Rpm{}.Match(ctx, "package.rpm", nil)
		if err != nil || !res.ByName {
			t.Errorf("Match by extension failed: res=%v err=%v", res, err)
		}
	})

	t.Run("By header", func(t *testing.T) {
		res, err := Rpm{}.Match(ctx, "", bytes.NewReader(rpmHeader))
		if err != nil || !res.ByStream {
			t.Errorf("Match by header failed: res=%v err=%v", res, err)
		}
	})

	t.Run("Invalid header", func(t *testing.T) {
		res, err := Rpm{}.Match(ctx, "", bytes.NewReader([]byte{0x00, 0x01, 0x02}))
		if err != nil || res.ByStream {
			t.Errorf("False positive match: res=%v err=%v", res, err)
		}
	})
}

// Test file testdata/test.rpm were created by:
//
// mkdir -p ~/rpmbuild/{BUILD,RPMS,SOURCES,SPECS}
// seq 0 2000 > ~/rpmbuild/SOURCES/test.txt
// cat > ~/rpmbuild/SPECS/test.spec <<EOF
// Name:           test
// Version:        1.0
// Release:        1%{?dist}
// Summary:        Test package containing test.txt
//
// License:        MIT
// Source0:        test.txt
//
// %description
// This is a test RPM package containing test.txt.
//
// %install
// mkdir -p %{buildroot}/opt/test
// install -m 644 %{SOURCE0} %{buildroot}/opt/test/
//
// %files
// /opt/test/test.txt
//
// %changelog
// * $(date '+%a %b %d %Y') Your Name <your.email@example.com> - 1.0-1
// - Initial package
// EOF
// rpmbuild -bb ~/rpmbuild/SPECS/test.spec
func TestRpm_Extract(t *testing.T) {
	const testRPM = "./testdata/test.rpm"
	rpmFile, err := os.Open(os.ExpandEnv(testRPM))
	if err != nil {
		t.Fatalf("Can't open test RPM: %v", err)
	}
	defer rpmFile.Close()

	ctx := context.Background()
	t.Run("Successful extraction", func(t *testing.T) {
		const expectedSHA1Sum = "4da7f88f69b44a3fdb705667019a65f4c6e058a3"
		handler := func(ctx context.Context, info FileInfo) error {
			if info.NameInArchive != "/opt/test/test.txt" {
				t.Errorf("Invalid file name in archive: got %s, want '/opt/test/test.txt'", info.NameInArchive)
			}
			if info.LinkTarget != "" {
				t.Errorf("Invalid link target: got %s, want ''", info.LinkTarget)
			}
			if info.Name() != "test.txt" {
				t.Errorf("Invalid file name: got %s, want 'test.txt'", info.Name())
			}
			if info.Size() != 8895 {
				t.Errorf("Invalid file size: got %d, want 8895", info.Size())
			}
			if info.ModTime() != time.Unix(1751149082, 0) {
				t.Errorf("Invalid mod time: got %s, want '2025-06-29 01:18:02'", info.ModTime())
			}
			if info.IsDir() {
				t.Errorf("Invalid is dir flag: got %t, want false", info.IsDir())
			}

			f, err := info.Open()
			if err != nil {
				return err
			}
			defer f.Close()

			h := sha1.New()
			if _, err = io.Copy(h, f); err != nil {
				return err
			}

			if got := hex.EncodeToString(h.Sum(nil)); got != expectedSHA1Sum {
				t.Errorf("Expected %s, got %s", expectedSHA1Sum, got)
			}

			return nil
		}

		r := Rpm{}
		if err := r.Extract(ctx, rpmFile, handler); err != nil {
			t.Fatalf("Extraction failed: %v", err)
		}
	})

	t.Run("ContinueOnError", func(t *testing.T) {
		r := Rpm{ContinueOnError: true}
		handler := func(ctx context.Context, f FileInfo) error {
			if strings.Contains(f.NameInArchive, "/opt/test/test.txt") {
				return errors.New("simulated error")
			}
			return nil
		}

		if _, err := rpmFile.Seek(0, 0); err != nil {
			t.Errorf("Failed to set offset: %v", err)
		}
		if err := r.Extract(ctx, rpmFile, handler); err != nil {
			t.Errorf("Should continue after errors: %v", err)
		}
	})
}

func TestRpmFileInfo(t *testing.T) {
	fi := &mockFileInfo{
		name:  "/test/file",
		size:  1024,
		mode:  0644,
		mtime: 1700000000,
	}

	rfi := rpmFileInfo{fi}
	if rfi.Name() != "file" {
		t.Errorf("Name() mismatch: got %s", rfi.Name())
	}
	if rfi.Size() != 1024 {
		t.Errorf("Size() mismatch")
	}
	if rfi.ModTime() != time.Unix(1700000000, 0) {
		t.Errorf("ModTime() mismatch")
	}
	if rfi.IsDir() {
		t.Errorf("IsDir() returned true for file")
	}
}

type mockFileInfo struct {
	name  string
	size  int64
	mode  int
	mtime int
	isDir bool
}

func (m *mockFileInfo) Name() string      { return m.name }
func (m *mockFileInfo) Size() int64       { return m.size }
func (m *mockFileInfo) UserName() string  { return "" }
func (m *mockFileInfo) GroupName() string { return "" }
func (m *mockFileInfo) Flags() int        { return 0 }
func (m *mockFileInfo) Mtime() int        { return m.mtime }
func (m *mockFileInfo) Digest() string    { return "" }
func (m *mockFileInfo) Mode() int         { return m.mode }
func (m *mockFileInfo) Linkname() string  { return "" }
func (m *mockFileInfo) Device() int       { return 0 }
func (m *mockFileInfo) Inode() int        { return 0 }
