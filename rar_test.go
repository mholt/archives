package archives

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"testing"
)

func TestRarExtractMultiVolume(t *testing.T) {
	// Test files testdata/test.part*.rar were created by:
	//   seq 0 2000 > test.txt
	//   rar a -v1k test.rar test.txt
	rar := Rar{
		Name: "test.part01.rar",
		FS:   DirFS("testdata"),
	}

	const expectedSHA1Sum = "4da7f88f69b44a3fdb705667019a65f4c6e058a3"
	if err := rar.Extract(context.Background(), nil, func(_ context.Context, info FileInfo) error {
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
			t.Errorf("expected %s, got %s", expectedSHA1Sum, got)
		}
		return nil
	}); err != nil {
		t.Error(err)
	}
}
