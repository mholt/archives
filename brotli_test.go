package archives

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

func TestBrotli_Match_Stream(t *testing.T) {
	testTxt := []byte("this is text, but it has to be long enough to match brotli which doesn't have a magic number")
	type testcase struct {
		name    string
		input   []byte
		matches bool
	}

	testCases := []testcase{
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
	}

	// Test all quality levels (0-11)
	for quality := 0; quality <= 11; quality++ {
		testCases = append(testCases, testcase{
			name:    fmt.Sprintf("text compressed with brotli quality %d", quality),
			input:   compress(t, ".br", testTxt, Brotli{Quality: quality}.OpenWriter),
			matches: true,
		})
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewBuffer(tc.input)

			mr, err := Brotli{}.Match(context.Background(), "", r)
			if err != nil {
				t.Errorf("Brotli.Match() error = %v", err)
				return
			}

			if mr.ByStream != tc.matches {
				t.Logf("input: %s", tc.input)
				t.Error("Brotli.Match() expected ByStream to be", tc.matches, "but got", mr.ByStream)
			}
		})
	}
}

func TestBrotli_Fuzzy_Both(t *testing.T) {
	// Use a deterministic seed for reproducible tests
	seed := int64(42)
	rng := &deterministicRNG{seed: seed}

	// Test both uncompressed ASCII and actual brotli compressed data
	numTests := 500
	for i := 0; i < numTests; i++ {
		// Generate random ASCII string of varying lengths
		length := rng.Intn(200) + 16
		asciiData := generateRandomASCII(rng, length)

		// Test uncompressed ASCII data (should not match)
		t.Run(fmt.Sprintf("ascii_%d", i), func(t *testing.T) {
			r := bytes.NewBuffer(asciiData)

			mr, err := Brotli{}.Match(context.Background(), "", r)
			if err != nil {
				t.Errorf("Brotli.Match() error = %v", err)
				return
			}

			if mr.ByStream {
				t.Errorf("Random ASCII data incorrectly detected as brotli compressed")
				t.Logf("Data: %q", string(asciiData))
				t.Logf("Length: %d", len(asciiData))
				t.Logf("Data bytes: %v", asciiData)
			}
		})

		// Test actual brotli compressed data (should match) - test all quality levels
		for quality := 0; quality <= 11; quality++ {
			t.Run(fmt.Sprintf("br_%d_q%d", i, quality), func(t *testing.T) {
				compressedData := compress(t, ".br", asciiData, Brotli{Quality: quality}.OpenWriter)

				r := bytes.NewBuffer(compressedData)

				mr, err := Brotli{}.Match(context.Background(), "", r)
				if err != nil {
					t.Errorf("Brotli.Match() error = %v", err)
					return
				}

				if !mr.ByStream {
					t.Errorf("Actual brotli compressed data not detected as compressed")
					t.Logf("Original data: %q", string(asciiData))
					t.Logf("Compressed length: %d", len(compressedData))
					t.Logf("Quality used: %d", quality)
					t.Logf("Compressed bytes: %v", compressedData[:min(32, len(compressedData))])
				}
			})
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// deterministicRNG provides deterministic random numbers for testing
type deterministicRNG struct {
	seed int64
}

func (r *deterministicRNG) Intn(n int) int {
	r.seed = (r.seed*1103515245 + 12345) & 0x7fffffff
	return int(r.seed % int64(n))
}

// generateRandomASCII creates a random ASCII string with common whitespace characters
func generateRandomASCII(rng *deterministicRNG, length int) []byte {
	// ASCII printable chars + whitespace: tab, newline, space, etc.
	chars := []byte(" \t\n\r\v\fabcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;':\",./<>?")

	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[rng.Intn(len(chars))]
	}
	return result
}
