package archives

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/sassoftware/go-rpmutils"
	"github.com/sassoftware/go-rpmutils/cpio"
)

func init() {
	RegisterFormat(Rpm{})
}

type Rpm struct {
	// If true, errors encountered during reading or writing
	// a file within an archive will be logged and the
	// operation will continue on remaining files.
	ContinueOnError bool
}

func (Rpm) Extension() string { return ".rpm" }
func (Rpm) MediaType() string { return "application/x-rpm" }

func (r Rpm) Match(_ context.Context, filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), r.Extension()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(rpmHeader))
	if err != nil {
		return mr, err
	}
	mr.ByStream = bytes.Equal(buf, rpmHeader)

	return mr, nil
}

func (r Rpm) Extract(ctx context.Context, sourceArchive io.Reader, handleFile FileHandler) error {
	rpm, err := rpmutils.ReadRpm(sourceArchive)
	if err != nil {
		return err
	}

	pre, err := rpm.PayloadReaderExtended()
	if err != nil {
		return err
	}

	// important to initialize to non-nil, empty value due to how fileIsIncluded works
	skipDirs := skipList{}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		f, err := pre.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if r.ContinueOnError && ctx.Err() == nil {
				log.Printf("[ERROR] Advancing to next file in rpm: %v", err)
				continue
			}
			return err
		}
		if fileIsIncluded(skipDirs, f.Name()) {
			continue
		}

		info := rpmFileInfo{f}
		file := FileInfo{
			FileInfo:      info,
			Header:        f,
			NameInArchive: f.Name(),
			Open: func() (fs.File, error) {
				return fileInArchive{io.NopCloser(pre), info}, nil
			},
		}

		err = handleFile(ctx, file)
		if errors.Is(err, fs.SkipAll) {
			break
		} else if errors.Is(err, fs.SkipDir) && file.IsDir() {
			skipDirs.add(f.Name())
		} else if err != nil {
			if r.ContinueOnError {
				log.Printf("[ERROR] %s: %v", f.Name(), err)
				continue
			}
			return fmt.Errorf("handling file: %s: %w", f.Name(), err)
		}
	}

	return nil
}

// rpmFileInfo satisfies the fs.FileInfo interface for RPM entries.
type rpmFileInfo struct {
	fi rpmutils.FileInfo
}

func (rfi rpmFileInfo) Name() string       { return path.Base(rfi.fi.Name()) }
func (rfi rpmFileInfo) Size() int64        { return rfi.fi.Size() }
func (rfi rpmFileInfo) Mode() os.FileMode  { return fs.FileMode(rfi.fi.Mode()) }
func (rfi rpmFileInfo) ModTime() time.Time { return time.Unix(int64(rfi.fi.Mtime()), 0) }
func (rfi rpmFileInfo) IsDir() bool        { return rfi.fi.Mode()&^07777 == cpio.S_ISDIR }
func (rfi rpmFileInfo) Sys() any           { return nil }

var rpmHeader = []byte{0x8e, 0xad, 0xe8, 0x01}
