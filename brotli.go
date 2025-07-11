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
	// the best way to match the stream is probably to try decoding part
	// of it, but we'll just have to guess a large-enough size that is
	// still small enough for the smallest streams we'll encounter
	input := &bytes.Buffer{}
	r := brotli.NewReader(io.TeeReader(stream, input))
	buf := make([]byte, 16)

	// First gauntlet - can the reader even read 16 bytes without an error?
	n, err := r.Read(buf)
	if err != nil {
		return false
	}
	buf = buf[:n]
	inputBytes := input.Bytes()

	// Second gauntlet - do the decompressed bytes exist in the raw input?
	// If they don't appear in the first 4 bytes (to account for the up to
	// 32 bits of initial brotli header) or at all, then chances are the
	// input was compressed.
	idx := bytes.Index(inputBytes, buf)
	if idx < 4 {
		return true
	}

	// The input is assumed to be compressed data, but we still can't be 100% sure.
	// Try reading more data until we encounter an error.
	for n < 128 {
		nn, err := r.Read(buf)
		switch err {
		case io.EOF:
			// If we've reached EOF, we return assuming it's compressed.
			return true
		case io.ErrUnexpectedEOF:
			// If we've encountered a short read, that's probably due to invalid reads due
			// to the fact it isn't compressed data at all.
			return false
		case nil:
			// No error, no problem. Continue reading.
			n += nn
		default:
			// If we encounter any other error, return it.
			return false
		}
	}

	// If we've read 128+ bytes without error, do a final ASCII check
	// Compressed data should not be pure ASCII
	inputBytes = input.Bytes()
	if isASCII(inputBytes) {
		return false
	}

	// If we got here, it seems like valid compressed data
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
