package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// run executes a git command in the given directory.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return string(out), nil
}

// ForcePull resets the local branch to match the remote, avoiding conflicts.
func ForcePull(repoDir string) error {
	if _, err := run(repoDir, "fetch", "--all"); err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	branch, err := run(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("rev-parse: %w", err)
	}
	branch = strings.TrimSpace(branch)
	if _, err := run(repoDir, "reset", "--hard", "origin/"+branch); err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	return nil
}

// CommitAndPush stages all changes, commits with the given message, and pushes.
func CommitAndPush(repoDir, message string) error {
	if _, err := run(repoDir, "add", "."); err != nil {
		return fmt.Errorf("add: %w", err)
	}
	if _, err := run(repoDir, "commit", "-m", message); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	if _, err := run(repoDir, "push"); err != nil {
		return fmt.Errorf("push: %w", err)
	}
	return nil
}
