package gitref

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGit runs a git command in dir, using -c flags for user identity so
// tests never touch the host's global git configuration. It fails the test
// on error.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func commit(t *testing.T, dir, message string) string {
	t.Helper()

	runGit(t, dir, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", message)
	return strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
}

// setupRepo builds a repository with two diverging branches:
//   - main: composer.lock created, then a second commit added.
//   - feature: branched off the first commit on main, composer.lock
//     removed, and a different file added instead.
//
// It returns the repo directory, the merge-base commit (the first commit
// on main, common ancestor of both branches), the main branch tip, and the
// feature branch tip.
func setupRepo(t *testing.T) (repoDir, mergeBase, mainTip, featureTip string) {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")

	writeFile(t, dir, "composer.lock", "{\"content\":\"v1\"}")
	runGit(t, dir, "add", "composer.lock")
	base := commit(t, dir, "Add composer.lock")

	runGit(t, dir, "checkout", "-b", "feature")
	runGit(t, dir, "rm", "composer.lock")
	writeFile(t, dir, "other.txt", "unrelated change")
	runGit(t, dir, "add", "other.txt")
	feature := commit(t, dir, "Remove composer.lock on feature branch")

	runGit(t, dir, "checkout", "main")
	writeFile(t, dir, "composer.lock", "{\"content\":\"v2\"}")
	runGit(t, dir, "add", "composer.lock")
	main := commit(t, dir, "Update composer.lock on main")

	return dir, base, main, feature
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestFileAtRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir, base, mainTip, featureTip := setupRepo(t)

	tests := []struct {
		name    string
		ref     string
		path    string
		want    string
		wantErr error
	}{
		{
			name: "content at merge base",
			ref:  base,
			path: "composer.lock",
			want: "{\"content\":\"v1\"}",
		},
		{
			name: "content at main tip",
			ref:  mainTip,
			path: "composer.lock",
			want: "{\"content\":\"v2\"}",
		},
		{
			name:    "path removed on feature branch",
			ref:     featureTip,
			path:    "composer.lock",
			wantErr: ErrFileNotFound,
		},
		{
			name:    "path never existed",
			ref:     mainTip,
			path:    "does-not-exist.txt",
			wantErr: ErrFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FileAtRef(dir, tt.ref, tt.path)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("FileAtRef() error = %v, want error wrapping %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("FileAtRef() unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("FileAtRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeBase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir, base, mainTip, featureTip := setupRepo(t)

	got, err := MergeBase(dir, mainTip, featureTip)
	if err != nil {
		t.Fatalf("MergeBase() unexpected error: %v", err)
	}
	if got != base {
		t.Errorf("MergeBase() = %q, want %q", got, base)
	}
}

func TestMergeBaseInvalidRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir, _, mainTip, _ := setupRepo(t)

	if _, err := MergeBase(dir, mainTip, "does-not-exist-ref"); err == nil {
		t.Fatal("MergeBase() expected error for invalid ref, got nil")
	}
}
