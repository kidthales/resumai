//go:build !windows

package mcp

import (
	"os"
	"syscall"
)

// writeFileNoFollow writes data to path with O_NOFOLLOW to close the leaf
// symlink race window on Unix platforms. Directory-segment traversal still
// assumes the local user's configured applications directory is not mutated
// concurrently by an untrusted actor.
func writeFileNoFollow(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, perm)
	if err != nil {
		return err
	}
	if _, werr := f.Write(data); werr != nil {
		_ = f.Close()
		return werr
	}
	return f.Close()
}
