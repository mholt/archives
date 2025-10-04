package extensions

import (
	"slices"
	"strings"
)

func EndsWith(path string, extension string) bool {
	extensions := strings.Split(strings.ToLower(path), ".")
	ext := strings.Trim(extension, ".")
	return extensions[len(extensions)-1] == ext
}

func Contains(path string, extension string) bool {
	extensions := strings.Split(strings.ToLower(path), ".")
	ext := strings.Trim(extension, ".")
	return slices.Contains(extensions, ext)
}
