//go:build !unix

package lock

// tryAcquire is a no-op on non-Unix platforms (development machines).
// Production deploys on Linux, where lock_unix.go uses a real flock.
func tryAcquire(path string) (func(), error) {
	return func() {}, nil
}
