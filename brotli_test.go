package archives

import (
	"bytes"
	"context"
	"testing"
)

func TestBrotli_Match_Stream(t *testing.T) {
	testTxt := []byte("this is text, but it has to be long enough to match brotli which doesn't have a magic number")
	type testcase struct {
		name    string
		input   []byte
		matches bool
	}
	for _, tc := range []testcase{
		{
			name:    "uncompressed yaml",
			input:   []byte("---\nthis-is-not-brotli: \"it is actually yaml\""),
			matches: false,
		},
		{
			name:    "uncompressed text",
			input:   testTxt,
			matches: false,
		},
		{
			name:    "text compressed with brotli quality 0",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 0}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 1",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 1}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 2",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 2}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 3",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 3}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 4",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 4}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 5",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 5}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 6",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 6}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 7",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 7}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 8",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 8}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 9",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 9}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 10",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 10}.OpenWriter),
			matches: true,
		},
		{
			name:    "text compressed with brotli quality 11",
			input:   compress(t, ".br", testTxt, Brotli{Quality: 11}.OpenWriter),
			matches: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewBuffer(tc.input)

			mr, err := Brotli{}.Match(context.Background(), "", r)
			if err != nil {
				t.Errorf("Brotli.OpenReader() error = %v", err)
				return
			}

			if mr.ByStream != tc.matches {
				t.Logf("input: %s", tc.input)
				t.Error("Brotli.Match() expected ByStream to be", tc.matches, "but got", mr.ByStream)
			}
		})
	}
}
