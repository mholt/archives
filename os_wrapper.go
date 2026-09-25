package archives

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// fixOSPath adds the \\?\ prefix on Windows to prevent the Win32 API
// from automatically stripping trailing dots and spaces from file names.
// On other systems it returns p unchanged.
func fixOSPath(p string) string {
	if runtime.GOOS != "windows" || p == "" {
		return p
	}
	if strings.HasPrefix(p, `\\?\`) {
		return p
	}
	abs, ok := absPathKeepingTrailingDots(p)
	if !ok {
		return p
	}
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + abs[2:]
	}
	return `\\?\` + abs
}

// absPathKeepingTrailingDots makes p absolute without calling filepath.Abs
// where possible: on Windows, filepath.Abs uses GetFullPathName, which itself
// strips trailing dots and spaces from path elements. filepath.Join and
// filepath.Clean are purely lexical and keep them.
func absPathKeepingTrailingDots(p string) (string, bool) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), true
	}
	if filepath.VolumeName(p) != "" || os.IsPathSeparator(p[0]) {
		// drive-relative ("C:foo") or rooted ("\foo") paths are rare;
		// let the OS resolve them
		abs, err := filepath.Abs(p)
		return abs, err == nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	return filepath.Join(wd, p), true
}
