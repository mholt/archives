package archives

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
)

func init() {
	RegisterFormat(Brotli{})
}

// Brotli facilitates brotli compression.
type Brotli struct {
	Quality int
}

func (Brotli) Extension() string { return ".br" }
func (Brotli) MediaType() string { return "application/x-br" }

func (br Brotli) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), br.Extension()) {
		mr.ByName = true
	}

	if stream != nil {
		mr.ByStream = isValidBrotliStream(stream)
	}

	return mr, nil
}

func isValidBrotliStream(stream io.Reader) bool {
	// brotli does not have well-defined file headers or a magic number;
	// the best way to match the stream is to try decoding a small amount
	// and see if it succeeds without errors
	input := &bytes.Buffer{}
	r := brotli.NewReader(io.TeeReader(stream, input))
	buf := make([]byte, 64)

	// Try to read some data - if it fails, it's likely not brotli
	n, err := r.Read(buf)
	if err != nil {
		return false
	}

	// Check if decompressed data appears in the raw input
	// If decompressed data is identical to input, it's likely uncompressed
	inputBytes := input.Bytes()
	if bytes.Equal(inputBytes, buf[:n]) {
		return false
	}

	// If we successfully decompressed data that's different from input,
	// and the input isn't pure ASCII, it's likely compressed
	if isASCII(inputBytes) {
		return false
	}

	return true
}

// isASCII checks if the given byte slice contains only ASCII printable characters and common whitespace.
// It allows:
// - Tab (9)
// - Newline (10)
// - Vertical tab (11)
// - Form feed (12)
// - Carriage return (13)
// - Space (32) through tilde (126) - all printable ASCII characters
// It excludes all other control characters and non-ASCII bytes.
func isASCII(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	for _, b := range data {
		if !isASCIIByte(b) {
			return false
		}
	}
	return true
}

func isASCIIByte(b byte) bool {
	// Allow tab, newline, vertical tab, form feed, carriage return
	if b >= 9 && b <= 13 {
		return true
	}
	// Allow space through tilde (printable ASCII)
	if b >= 32 && b <= 126 {
		return true
	}
	return false
}

func (br Brotli) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return brotli.NewWriterLevel(w, br.Quality), nil
}

func (Brotli) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(brotli.NewReader(r)), nil
}
