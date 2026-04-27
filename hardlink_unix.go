//go:build !windows

package archives

import (
	"io/fs"
	"syscall"
)

type inodeKey struct {
	dev uint64
	ino uint64
}

// hardlinkKey returns an inodeKey and true if info is a regular file with
// multiple hard links. Returns false on systems where Stat_t is unavailable
// or when the file has only one link.
func hardlinkKey(info fs.FileInfo) (inodeKey, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink <= 1 {
		return inodeKey{}, false
	}
	return inodeKey{dev: uint64(stat.Dev), ino: stat.Ino}, true
}
