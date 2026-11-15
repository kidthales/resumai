//go:build windows

package mcp

import "os"

// writeFileNoFollow falls back to os.WriteFile on Windows after Lstat symlink
// checks in ensureWritableForApp, because Windows lacks O_NOFOLLOW.
func writeFileNoFollow(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
