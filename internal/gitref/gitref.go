// Package gitref is a thin, testable wrapper around the `git` CLI for the
// two operations the orchestrator needs: reading a file's content at a
// given ref, and finding the merge-base commit between two refs.
package gitref

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrFileNotFound indicates the requested path did not exist at the given ref.
var ErrFileNotFound = errors.New("gitref: file not found at ref")

// gitCommand builds an exec.Cmd for git, forcing a C locale so error
// messages on stderr are in English and can be matched reliably regardless
// of the host's locale configuration.
func gitCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	return cmd
}

// pathMissing reports whether git's stderr output for `git show ref:path`
// indicates the path did not exist at the given ref. Git reports this in at
// least two forms depending on version and whether the path happens to
// exist in the current worktree on disk:
//   - "fatal: path 'X' does not exist in 'Y'"
//   - "fatal: path 'X' exists on disk, but not in 'Y'"
func pathMissing(stderrText string) bool {
	return strings.Contains(stderrText, "does not exist") ||
		strings.Contains(stderrText, "exists on disk, but not in")
}

// FileAtRef returns the content of path as it existed at ref, within the git
// repository rooted at repoDir (runs `git -C repoDir show ref:path`).
// If path did not exist at ref, the returned error wraps ErrFileNotFound
// (checkable with errors.Is).
func FileAtRef(repoDir, ref, path string) ([]byte, error) {
	cmd := gitCommand("-C", repoDir, "show", ref+":"+path)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrText := stderr.String()
		if pathMissing(stderrText) {
			return nil, fmt.Errorf("git show %s:%s: %w", ref, path, ErrFileNotFound)
		}
		return nil, fmt.Errorf("git show %s:%s: %w (%s)", ref, path, err, strings.TrimSpace(stderrText))
	}

	return stdout.Bytes(), nil
}

// MergeBase returns the merge-base commit SHA between refA and refB, within
// the git repository rooted at repoDir (runs `git -C repoDir merge-base refA refB`).
func MergeBase(repoDir, refA, refB string) (string, error) {
	cmd := gitCommand("-C", repoDir, "merge-base", refA, refB)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git merge-base %s %s: %w (%s)", refA, refB, err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}
