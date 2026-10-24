// Package lock provides a best-effort process lock to prevent overlapping
// runs of the bot (e.g. a cron job starting while a previous run is stuck).
package lock

// TryAcquire takes an exclusive lock on path and returns a release function.
// It fails immediately (without blocking) if another process holds the lock.
// On non-Unix platforms it is a no-op that always succeeds; production runs
// on Linux where a real flock is used.
func TryAcquire(path string) (func(), error) {
	return tryAcquire(path)
}
