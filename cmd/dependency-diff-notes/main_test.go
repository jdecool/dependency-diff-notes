package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jdecool/dependency-diff-notes/internal/config"
	"github.com/jdecool/dependency-diff-notes/internal/github"
	"github.com/jdecool/dependency-diff-notes/internal/gitlab"
	"github.com/jdecool/dependency-diff-notes/internal/report"
)

func TestDecideCommentAction(t *testing.T) {
	tests := []struct {
		name                 string
		diffIsEmpty          bool
		existingCommentFound bool
		want                 commentAction
	}{
		{
			name:                 "no existing comment, empty diff",
			diffIsEmpty:          true,
			existingCommentFound: false,
			want:                 noAction,
		},
		{
			name:                 "no existing comment, non-empty diff",
			diffIsEmpty:          false,
			existingCommentFound: false,
			want:                 createAction,
		},
		{
			name:                 "existing comment, empty diff",
			diffIsEmpty:          true,
			existingCommentFound: true,
			want:                 updateAction,
		},
		{
			name:                 "existing comment, non-empty diff",
			diffIsEmpty:          false,
			existingCommentFound: true,
			want:                 updateAction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideCommentAction(tt.diffIsEmpty, tt.existingCommentFound)
			if got != tt.want {
				t.Errorf("decideCommentAction(%v, %v) = %v, want %v", tt.diffIsEmpty, tt.existingCommentFound, got, tt.want)
			}
		})
	}
}

func TestNewForgeClient(t *testing.T) {
	tests := []struct {
		name  string
		forge config.Forge
		want  string
	}{
		{name: "GitLab", forge: config.GitLab, want: fmt.Sprintf("%T", &gitlab.Client{})},
		{name: "GitHub", forge: config.GitHub, want: fmt.Sprintf("%T", &github.Client{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newForgeClient(config.Config{Forge: tt.forge})
			if gotType := fmt.Sprintf("%T", got); gotType != tt.want {
				t.Errorf("newForgeClient(Forge: %v) type = %s, want %s", tt.forge, gotType, tt.want)
			}
		})
	}
}

// --- end-to-end tests ---

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

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// setupOrchestrationRepo builds a repository simulating a Change Request:
//   - a bare "remote" repo, playing the role of the Forge-hosted origin.
//   - a working clone whose "main" branch has one commit (baseLock as
//     composer.lock), pushed and fetched to origin/main.
//   - a second commit on top (headLock as composer.lock), left unpushed,
//     playing the role of the Change Request's current HEAD.
//
// It returns the working clone's directory.
func setupOrchestrationRepo(t *testing.T, baseLock, headLock string) string {
	t.Helper()

	remoteDir := t.TempDir()
	runGit(t, remoteDir, "init", "--bare", "-b", "main")

	workDir := t.TempDir()
	runGit(t, workDir, "init", "-b", "main")
	runGit(t, workDir, "remote", "add", "origin", remoteDir)

	writeFile(t, workDir, "composer.lock", baseLock)
	runGit(t, workDir, "add", "composer.lock")
	runGit(t, workDir, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "Base commit")

	runGit(t, workDir, "push", "origin", "main")
	runGit(t, workDir, "fetch", "origin")

	writeFile(t, workDir, "composer.lock", headLock)
	runGit(t, workDir, "add", "composer.lock")
	runGit(t, workDir, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "Update composer.lock")

	return workDir
}

const (
	testBaseLock = `{"packages":[{"name":"vendor/pkg1","version":"1.0.0"}],"packages-dev":[]}`
	testHeadLock = `{"packages":[{"name":"vendor/pkg1","version":"1.0.0"},{"name":"vendor/pkg2","version":"2.0.0"}],"packages-dev":[]}`
)

// setupMultiFileOrchestrationRepo behaves like setupOrchestrationRepo, but
// commits an arbitrary set of files (path -> content) at each stage, so
// more than one Ecosystem's Lockfile can be present in the same repo.
func setupMultiFileOrchestrationRepo(t *testing.T, baseFiles, headFiles map[string]string) string {
	t.Helper()

	remoteDir := t.TempDir()
	runGit(t, remoteDir, "init", "--bare", "-b", "main")

	workDir := t.TempDir()
	runGit(t, workDir, "init", "-b", "main")
	runGit(t, workDir, "remote", "add", "origin", remoteDir)

	for name, content := range baseFiles {
		writeFile(t, workDir, name, content)
		runGit(t, workDir, "add", name)
	}
	runGit(t, workDir, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "Base commit")

	runGit(t, workDir, "push", "origin", "main")
	runGit(t, workDir, "fetch", "origin")

	for name, content := range headFiles {
		writeFile(t, workDir, name, content)
		runGit(t, workDir, "add", name)
	}
	runGit(t, workDir, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "Update lockfiles")

	return workDir
}

// commentJSON is the wire shape shared by both fake Forge servers below
// (GitLab notes and GitHub issue comments both expose "id" and "body").
type commentJSON struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

// fakeGitLabServer spins up an httptest.Server acting as the GitLab API for
// a single project/merge request, recording the notes-related calls it
// receives. existingNotes is served as the response to ListNotes.
type fakeGitLabServer struct {
	*httptest.Server

	listNotesCalled bool
	createCalled    bool
	createBody      string
	updateCalled    bool
	updateNoteID    string
	updateBody      string
}

func newFakeGitLabServer(t *testing.T, projectID, mrIID string, existingNotes []commentJSON) *fakeGitLabServer {
	t.Helper()

	f := &fakeGitLabServer{}

	notesPath := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%s/notes", projectID, mrIID)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+notesPath, func(w http.ResponseWriter, r *http.Request) {
		f.listNotesCalled = true
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(existingNotes); err != nil {
			t.Fatalf("encode notes response: %v", err)
		}
	})
	mux.HandleFunc("POST "+notesPath, func(w http.ResponseWriter, r *http.Request) {
		f.createCalled = true
		var payload struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode create note request: %v", err)
		}
		f.createBody = payload.Body

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(commentJSON{ID: 99, Body: payload.Body}); err != nil {
			t.Fatalf("encode create note response: %v", err)
		}
	})
	mux.HandleFunc("PUT "+notesPath+"/{noteID}", func(w http.ResponseWriter, r *http.Request) {
		f.updateCalled = true
		f.updateNoteID = r.PathValue("noteID")

		var payload struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode update note request: %v", err)
		}
		f.updateBody = payload.Body

		w.WriteHeader(http.StatusOK)
	})

	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Server.Close)

	return f
}

func runArgs(repoDir, serverURL, projectID, mrIID string) []string {
	return []string{
		"--server-url", serverURL,
		"--project-id", projectID,
		"--request-iid", mrIID,
		"--target-branch", "main",
		"--token", "test-token",
		"--repo-dir", repoDir,
	}
}

func TestRunCreatesCommentWhenNoneExists(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Pin the Forge to GitLab: on a GitHub Actions runner GITHUB_ACTIONS=true
	// would otherwise make config.Detect() pick GitHub and ignore the fake
	// GitLab server below.
	t.Setenv("GITHUB_ACTIONS", "")

	repoDir := setupOrchestrationRepo(t, testBaseLock, testHeadLock)
	server := newFakeGitLabServer(t, "123", "45", nil)

	if err := run(runArgs(repoDir, server.URL, "123", "45")); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if !server.listNotesCalled {
		t.Error("expected ListNotes to be called")
	}
	if !server.createCalled {
		t.Fatal("expected CreateNote to be called")
	}
	if server.updateCalled {
		t.Error("expected UpdateNote not to be called")
	}
	if !strings.Contains(server.createBody, "vendor/pkg2") || !strings.Contains(server.createBody, "2.0.0") {
		t.Errorf("create note body = %q, want it to mention vendor/pkg2 2.0.0", server.createBody)
	}
	if !report.HasMarker(server.createBody) {
		t.Errorf("create note body = %q, want it to contain the bot marker", server.createBody)
	}
}

func TestRunUpdatesExistingComment(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Pin the Forge to GitLab: on a GitHub Actions runner GITHUB_ACTIONS=true
	// would otherwise make config.Detect() pick GitHub and ignore the fake
	// GitLab server below.
	t.Setenv("GITHUB_ACTIONS", "")

	repoDir := setupOrchestrationRepo(t, testBaseLock, testHeadLock)
	existing := []commentJSON{
		{ID: 7, Body: report.Marker + "\n## Dependency changes\n\nNo dependency changes detected.\n"},
	}
	server := newFakeGitLabServer(t, "123", "45", existing)

	if err := run(runArgs(repoDir, server.URL, "123", "45")); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if !server.listNotesCalled {
		t.Error("expected ListNotes to be called")
	}
	if server.createCalled {
		t.Error("expected CreateNote not to be called")
	}
	if !server.updateCalled {
		t.Fatal("expected UpdateNote to be called")
	}
	if server.updateNoteID != "7" {
		t.Errorf("update note ID = %q, want %q", server.updateNoteID, "7")
	}
	if !strings.Contains(server.updateBody, "vendor/pkg2") || !strings.Contains(server.updateBody, "2.0.0") {
		t.Errorf("update note body = %q, want it to mention vendor/pkg2 2.0.0", server.updateBody)
	}
}

func TestRunReportsBothEcosystemsInOneComment(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Pin the Forge to GitLab: on a GitHub Actions runner GITHUB_ACTIONS=true
	// would otherwise make config.Detect() pick GitHub and ignore the fake
	// GitLab server below.
	t.Setenv("GITHUB_ACTIONS", "")

	baseFiles := map[string]string{
		"composer.lock":     testBaseLock,
		"package-lock.json": `{"packages":{"":{"name":"my-app"},"node_modules/lodash":{"version":"4.17.20","resolved":"https://registry.npmjs.org/lodash/-/lodash-4.17.20.tgz"}}}`,
	}
	headFiles := map[string]string{
		"composer.lock":     testHeadLock,
		"package-lock.json": `{"packages":{"":{"name":"my-app"},"node_modules/lodash":{"version":"4.17.21","resolved":"https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz"}}}`,
	}

	repoDir := setupMultiFileOrchestrationRepo(t, baseFiles, headFiles)
	server := newFakeGitLabServer(t, "123", "45", nil)

	if err := run(runArgs(repoDir, server.URL, "123", "45")); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if !server.createCalled {
		t.Fatal("expected CreateNote to be called")
	}
	if !strings.Contains(server.createBody, "### Composer") || !strings.Contains(server.createBody, "### npm") {
		t.Errorf("create note body = %q, want it to contain both a Composer and an npm section", server.createBody)
	}
	if !strings.Contains(server.createBody, "vendor/pkg2") {
		t.Errorf("create note body = %q, want it to mention the Composer change vendor/pkg2", server.createBody)
	}
	if !strings.Contains(server.createBody, "lodash") {
		t.Errorf("create note body = %q, want it to mention the npm change lodash", server.createBody)
	}
}

func TestRunNoOpOutsideChangeRequestContext(t *testing.T) {
	// Make sure no ambient CI environment leaks a change request context
	// into this test.
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI_MERGE_REQUEST_IID", "")

	// No server is started at all: if run() attempted any HTTP call it
	// would fail outright, since --server-url isn't even set.
	if err := run(nil); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}
}
