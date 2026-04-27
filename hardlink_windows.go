//go:build windows

package archives

import "io/fs"

type inodeKey struct{}

func hardlinkKey(_ fs.FileInfo) (inodeKey, bool) {
	return inodeKey{}, false
}
