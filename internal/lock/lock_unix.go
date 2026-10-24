//go:build unix

package lock

import (
	"fmt"
	"os"
	"syscall"
)

func tryAcquire(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another run holds the lock: %w", err)
	}
	// Record the PID for debugging; harmless if the write fails.
	f.Truncate(0)
	fmt.Fprintf(f, "%d\n", os.Getpid())

	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
