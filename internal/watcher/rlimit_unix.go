//go:build !windows

package watcher

import "syscall"

// processFDLimit returns the soft limit on open file descriptors for this
// process, or 0 if it cannot be determined.
func processFDLimit() uint64 {
	var retval uint64

	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err == nil {
		retval = uint64(limit.Cur)
	}

	return retval
}
