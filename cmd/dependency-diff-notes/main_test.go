package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func TestDecideAction(t *testing.T) {
	tests := []struct {
		name        string
		present     bool
		diffIsEmpty bool
		unchanged   bool
		want        reportAction
	}{
		{
			name:        "nothing published, empty diff",
			present:     false,
			diffIsEmpty: true,
			want:        noAction,
		},
		{
			name:        "nothing published, non-empty diff",
			present:     false,
			diffIsEmpty: false,
			want:        createAction,
		},
		{
			name:        "already published, empty diff, differing content",
			present:     true,
			diffIsEmpty: true,
			unchanged:   false,
			want:        updateAction,
		},
		{
			name:        "already published, non-empty diff, differing content",
			present:     true,
			diffIsEmpty: false,
			unchanged:   false,
			want:        updateAction,
		},
		{
			name:        "already published, identical content",
			present:     true,
			diffIsEmpty: false,
			unchanged:   true,
			want:        noAction,
		},
		{
			name:        "already published, identical content, empty diff",
			present:     true,
			diffIsEmpty: true,
			unchanged:   true,
			want:        noAction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideAction(tt.present, tt.diffIsEmpty, tt.unchanged)
			if got != tt.want {
				t.Errorf("decideAction(%v, %v, %v) = %v, want %v", tt.present, tt.diffIsEmpty, tt.unchanged, got, tt.want)
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
// a single project/merge request, recording the calls it receives on both
// Report Destinations (see CONTEXT.md). existingNotes is served as the
// response to ListNotes; description holds the merge request description and
// may be set by a test before calling run().
type fakeGitLabServer struct {
	*httptest.Server

	listNotesCalled bool
	createCalled    bool
	createBody      string
	updateCalled    bool
	updateNoteID    string
	updateBody      string
	deleteCalled    bool
	deleteNoteID    string

	description        string
	descriptionUpdated bool
}

func newFakeGitLabServer(t *testing.T, projectID, mrIID string, existingNotes []commentJSON) *fakeGitLabServer {
	t.Helper()

	f := &fakeGitLabServer{}

	notesPath := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%s/notes", projectID, mrIID)
	mrPath := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%s", projectID, mrIID)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+mrPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"description": f.description}); err != nil {
			t.Fatalf("encode merge request response: %v", err)
		}
	})
	mux.HandleFunc("PUT "+mrPath, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode update merge request request: %v", err)
		}

		f.descriptionUpdated = true
		f.description = payload.Description

		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("DELETE "+notesPath+"/{noteID}", func(w http.ResponseWriter, r *http.Request) {
		f.deleteCalled = true
		f.deleteNoteID = r.PathValue("noteID")

		w.WriteHeader(http.StatusNoContent)
	})
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

	if err := run(runArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
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

	if err := run(runArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
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

	if err := run(runArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
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

func TestRunReportsPnpmCombinedSection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Pin the Forge to GitLab: on a GitHub Actions runner GITHUB_ACTIONS=true
	// would otherwise make config.Detect() pick GitHub and ignore the fake
	// GitLab server below.
	t.Setenv("GITHUB_ACTIONS", "")

	baseFiles := map[string]string{
		"pnpm-lock.yaml": "lockfileVersion: '9.0'\n" +
			"importers:\n" +
			"  .:\n" +
			"    dependencies:\n" +
			"      lodash:\n" +
			"        specifier: ^4.17.20\n" +
			"        version: 4.17.20\n" +
			"packages:\n" +
			"  lodash@4.17.20:\n" +
			"    resolution: {integrity: sha512-example==}\n",
	}
	headFiles := map[string]string{
		"pnpm-lock.yaml": "lockfileVersion: '9.0'\n" +
			"importers:\n" +
			"  .:\n" +
			"    dependencies:\n" +
			"      lodash:\n" +
			"        specifier: ^4.17.21\n" +
			"        version: 4.17.21\n" +
			"packages:\n" +
			"  lodash@4.17.21:\n" +
			"    resolution: {integrity: sha512-example==}\n",
	}

	repoDir := setupMultiFileOrchestrationRepo(t, baseFiles, headFiles)
	server := newFakeGitLabServer(t, "123", "45", nil)

	if err := run(runArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if !server.createCalled {
		t.Fatal("expected CreateNote to be called")
	}
	if !strings.Contains(server.createBody, "### pnpm") {
		t.Errorf("create note body = %q, want it to contain a pnpm section", server.createBody)
	}
	if !strings.Contains(server.createBody, "<summary>Dependencies (1)</summary>") {
		t.Errorf("create note body = %q, want a single undifferentiated Dependencies group (lockfileVersion 9.0 has no dev/prod split)", server.createBody)
	}
	if strings.Contains(server.createBody, "Production dependencies") || strings.Contains(server.createBody, "Development dependencies") {
		t.Errorf("create note body = %q, want no Production/Development split for pnpm lockfileVersion 9.0", server.createBody)
	}
	if !strings.Contains(server.createBody, "lodash") {
		t.Errorf("create note body = %q, want it to mention the pnpm change lodash", server.createBody)
	}
}

func TestRunReportsYarnCombinedSection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Pin the Forge to GitLab: on a GitHub Actions runner GITHUB_ACTIONS=true
	// would otherwise make config.Detect() pick GitHub and ignore the fake
	// GitLab server below.
	t.Setenv("GITHUB_ACTIONS", "")

	classicYarnLock := func(version string) string {
		return "# THIS IS AN AUTOGENERATED FILE. DO NOT EDIT THIS FILE DIRECTLY.\n" +
			"# yarn lockfile v1\n\n" +
			"lodash@^4.17.20:\n" +
			"  version \"" + version + "\"\n" +
			"  resolved \"https://registry.yarnpkg.com/lodash/-/lodash-" + version + ".tgz#exampleshasum\"\n"
	}

	baseFiles := map[string]string{"yarn.lock": classicYarnLock("4.17.20")}
	headFiles := map[string]string{"yarn.lock": classicYarnLock("4.17.21")}

	repoDir := setupMultiFileOrchestrationRepo(t, baseFiles, headFiles)
	server := newFakeGitLabServer(t, "123", "45", nil)

	if err := run(runArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if !server.createCalled {
		t.Fatal("expected CreateNote to be called")
	}
	if !strings.Contains(server.createBody, "### Yarn") {
		t.Errorf("create note body = %q, want it to contain a Yarn section", server.createBody)
	}
	if !strings.Contains(server.createBody, "<summary>Dependencies (1)</summary>") {
		t.Errorf("create note body = %q, want a single undifferentiated Dependencies group (yarn.lock has no dev/prod split)", server.createBody)
	}
	if strings.Contains(server.createBody, "Production dependencies") || strings.Contains(server.createBody, "Development dependencies") {
		t.Errorf("create note body = %q, want no Production/Development split for Yarn", server.createBody)
	}
	if !strings.Contains(server.createBody, "lodash") {
		t.Errorf("create note body = %q, want it to mention the Yarn change lodash", server.createBody)
	}
}

func TestRunFailsOnConflictingJSLockfiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Pin the Forge to GitLab: on a GitHub Actions runner GITHUB_ACTIONS=true
	// would otherwise make config.Detect() pick GitHub and ignore the fake
	// GitLab server below.
	t.Setenv("GITHUB_ACTIONS", "")

	// Both package-lock.json and pnpm-lock.yaml present at the same ref
	// (HEAD): a genuine conflict (see CONTEXT.md: Lockfile), not the
	// legitimate multi-Ecosystem case Composer + npm is. Neither exists yet
	// at the base commit, so the two commits actually differ (an identical
	// base/head commit would be an empty, rejected commit).
	pkgLock := `{"packages":{"":{"name":"my-app"},"node_modules/lodash":{"version":"4.17.21","resolved":"https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz"}}}`
	pnpmLock := "lockfileVersion: '9.0'\npackages:\n  lodash@4.17.21:\n    resolution: {integrity: sha512-example==}\n"

	baseFiles := map[string]string{"README.md": "base"}
	headFiles := map[string]string{
		"README.md":         "base",
		"package-lock.json": pkgLock,
		"pnpm-lock.yaml":    pnpmLock,
	}

	repoDir := setupMultiFileOrchestrationRepo(t, baseFiles, headFiles)
	server := newFakeGitLabServer(t, "123", "45", nil)

	err := run(runArgs(repoDir, server.URL, "123", "45"), io.Discard)
	if err == nil {
		t.Fatal("run() error = nil, want a conflict error")
	}
	if !strings.Contains(err.Error(), "package-lock.json") || !strings.Contains(err.Error(), "pnpm-lock.yaml") {
		t.Errorf("run() error = %q, want it to mention both conflicting Lockfiles", err.Error())
	}
	if server.createCalled || server.updateCalled {
		t.Error("expected no Bot Comment to be created or updated when the run fails on a conflict")
	}
}

func TestRunAllowlistDefusesConflictingJSLockfiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Pin the Forge to GitLab: on a GitHub Actions runner GITHUB_ACTIONS=true
	// would otherwise make config.Detect() pick GitHub and ignore the fake
	// GitLab server below.
	t.Setenv("GITHUB_ACTIONS", "")

	// Same setup as TestRunFailsOnConflictingJSLockfiles: both package-lock.json
	// and pnpm-lock.yaml present at HEAD. Restricting the Considered Ecosystems
	// to pnpm drops npm for the whole run, so the conflict never arises and only
	// the pnpm section is reported.
	pkgLock := `{"packages":{"":{"name":"my-app"},"node_modules/lodash":{"version":"4.17.21","resolved":"https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz"}}}`
	pnpmLock := "lockfileVersion: '9.0'\npackages:\n  lodash@4.17.21:\n    resolution: {integrity: sha512-example==}\n"

	baseFiles := map[string]string{"README.md": "base"}
	headFiles := map[string]string{
		"README.md":         "base",
		"package-lock.json": pkgLock,
		"pnpm-lock.yaml":    pnpmLock,
	}

	repoDir := setupMultiFileOrchestrationRepo(t, baseFiles, headFiles)
	server := newFakeGitLabServer(t, "123", "45", nil)

	args := append(runArgs(repoDir, server.URL, "123", "45"), "--ecosystems", "pnpm")
	if err := run(args, io.Discard); err != nil {
		t.Fatalf("run() unexpected error with pnpm allowlisted: %v", err)
	}

	if !server.createCalled {
		t.Fatal("expected CreateNote to be called")
	}
	if !strings.Contains(server.createBody, "### pnpm") {
		t.Errorf("create note body = %q, want a pnpm section", server.createBody)
	}
	if strings.Contains(server.createBody, "### npm") {
		t.Errorf("create note body = %q, want npm to be excluded by the allowlist", server.createBody)
	}
}

func TestRunNoOpOutsideChangeRequestContext(t *testing.T) {
	// Make sure no ambient CI environment leaks a change request context
	// into this test.
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI_MERGE_REQUEST_IID", "")

	// No server is started at all: if run() attempted any HTTP call it
	// would fail outright, since --server-url isn't even set.
	if err := run(nil, io.Discard); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}
}

// --- Local Comparison (see CONTEXT.md) end-to-end tests ---

// clearChangeRequestEnv makes sure no ambient CI environment leaks a Change
// Request context into a Local Comparison test, which would otherwise flip
// run() into CI mode and attempt to reach a Forge.
func clearChangeRequestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI_MERGE_REQUEST_IID", "")
	t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "")
	t.Setenv("GITHUB_BASE_REF", "")
}

// initLocalRepo initializes a git repository on "main" with a single commit
// containing composer.lock == baseLock, and returns its directory. It needs
// no remote: a Local Comparison resolves refs locally.
func initLocalRepo(t *testing.T, baseLock string) string {
	t.Helper()

	workDir := t.TempDir()
	runGit(t, workDir, "init", "-b", "main")
	writeFile(t, workDir, "composer.lock", baseLock)
	runGit(t, workDir, "add", "composer.lock")
	runGit(t, workDir, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "Base commit")

	return workDir
}

func TestRunLocalComparisonReadsWorkingTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	clearChangeRequestEnv(t)

	// main is committed with pkg1 only; the working tree adds pkg2 but is
	// never committed. The default Source (empty) must read this on-disk state.
	workDir := initLocalRepo(t, testBaseLock)
	writeFile(t, workDir, "composer.lock", testHeadLock)

	var buf bytes.Buffer
	args := []string{"--target-branch", "main", "--repo-dir", workDir}
	if err := run(args, &buf); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "+ vendor/pkg2") {
		t.Errorf("output = %q, want a '+' addition marker for the uncommitted working-tree change vendor/pkg2", out)
	}
	if report.HasMarker(out) {
		t.Errorf("output = %q, want no hidden Bot Comment marker in local terminal output", out)
	}
}

func TestRunLocalComparisonSourceRefIgnoresWorkingTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	clearChangeRequestEnv(t)

	// main has pkg1; a feature branch commits pkg2; the working tree then adds
	// an uncommitted pkg3. With --source HEAD the comparison must read the
	// feature commit (pkg2), not the working tree (pkg3).
	workDir := initLocalRepo(t, testBaseLock)
	runGit(t, workDir, "checkout", "-b", "feature")
	writeFile(t, workDir, "composer.lock", testHeadLock)
	runGit(t, workDir, "add", "composer.lock")
	runGit(t, workDir, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "Feature commit")

	pkg3Lock := `{"packages":[{"name":"vendor/pkg1","version":"1.0.0"},{"name":"vendor/pkg2","version":"2.0.0"},{"name":"vendor/pkg3","version":"3.0.0"}],"packages-dev":[]}`
	writeFile(t, workDir, "composer.lock", pkg3Lock)

	var buf bytes.Buffer
	args := []string{"--target-branch", "main", "--source", "HEAD", "--repo-dir", workDir}
	if err := run(args, &buf); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "vendor/pkg2") {
		t.Errorf("output = %q, want it to report the committed change vendor/pkg2", out)
	}
	if strings.Contains(out, "vendor/pkg3") {
		t.Errorf("output = %q, want the uncommitted vendor/pkg3 to be ignored when --source is a ref", out)
	}
}

func TestRunLocalComparisonNoChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	clearChangeRequestEnv(t)

	// The working tree matches the committed base: no Dependency Changes.
	workDir := initLocalRepo(t, testBaseLock)

	var buf bytes.Buffer
	args := []string{"--target-branch", "main", "--repo-dir", workDir}
	if err := run(args, &buf); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if out := buf.String(); !strings.Contains(out, "No dependency changes detected.") {
		t.Errorf("output = %q, want it to report no dependency changes", out)
	}
}

// --- Report Destination (see CONTEXT.md) end-to-end tests ---

// descriptionArgs is runArgs with the description Report Destination selected.
func descriptionArgs(repoDir, serverURL, projectID, mrIID string) []string {
	return append(runArgs(repoDir, serverURL, projectID, mrIID), "--report-destination", "description")
}

func TestRunPublishesReportInDescriptionAndLeavesAuthorTextAlone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("GITHUB_ACTIONS", "")

	repoDir := setupOrchestrationRepo(t, testBaseLock, testHeadLock)
	server := newFakeGitLabServer(t, "123", "45", nil)
	server.description = "Bumps the vendored packages.\n\nCloses #12.\n"

	if err := run(descriptionArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if !server.descriptionUpdated {
		t.Fatal("expected the merge request description to be updated")
	}
	if server.createCalled {
		t.Error("expected no Bot Comment to be created when the report goes to the description")
	}
	if !strings.HasPrefix(server.description, "Bumps the vendored packages.\n\nCloses #12.\n") {
		t.Errorf("description = %q, want the author's text preserved verbatim at the front", server.description)
	}
	if !strings.Contains(server.description, "vendor/pkg2") {
		t.Errorf("description = %q, want it to report vendor/pkg2", server.description)
	}
	if !strings.Contains(server.description, report.EndMarker) {
		t.Errorf("description = %q, want the region to be closed by the end marker", server.description)
	}
}

func TestRunDescriptionModeDeletesTheStaleBotComment(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("GITHUB_ACTIONS", "")

	// A Bot Comment left over from before the operator switched destination:
	// it would never be updated again, so it must not survive next to a live
	// report in the description.
	repoDir := setupOrchestrationRepo(t, testBaseLock, testHeadLock)
	existing := []commentJSON{
		{ID: 7, Body: report.Marker + "\n## Dependency changes\n\nstale\n" + report.EndMarker + "\n"},
	}
	server := newFakeGitLabServer(t, "123", "45", existing)

	if err := run(descriptionArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if !server.deleteCalled {
		t.Fatal("expected the stale Bot Comment to be deleted")
	}
	if server.deleteNoteID != "7" {
		t.Errorf("deleted note ID = %q, want %q", server.deleteNoteID, "7")
	}
	if server.updateCalled {
		t.Error("expected the stale Bot Comment to be deleted, not updated")
	}
	if !strings.Contains(server.description, "vendor/pkg2") {
		t.Errorf("description = %q, want the report to have moved there", server.description)
	}
}

func TestRunCommentModeStripsTheStaleDescriptionRegion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("GITHUB_ACTIONS", "")

	// The mirror image: a region left in the description after switching back
	// to the Bot Comment.
	repoDir := setupOrchestrationRepo(t, testBaseLock, testHeadLock)
	server := newFakeGitLabServer(t, "123", "45", nil)
	server.description = "Bumps the vendored packages.\n\n" +
		report.Marker + "\n## Dependency changes\n\nstale\n" + report.EndMarker + "\n"

	if err := run(runArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if !server.createCalled {
		t.Fatal("expected the Bot Comment to be created")
	}
	if report.HasMarker(server.description) {
		t.Errorf("description = %q, want the stale region stripped out", server.description)
	}
	if !strings.HasPrefix(server.description, "Bumps the vendored packages.\n") {
		t.Errorf("description = %q, want the author's text preserved", server.description)
	}
}

func TestRunWritesNothingWhenTheReportIsUnchanged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("GITHUB_ACTIONS", "")

	// Two consecutive runs on an unchanged Change Request: the second must
	// make no write at all, so the bot does not fill the activity feed with
	// "changed the description" on every pipeline run.
	repoDir := setupOrchestrationRepo(t, testBaseLock, testHeadLock)
	server := newFakeGitLabServer(t, "123", "45", nil)
	server.description = "Bumps the vendored packages.\n"

	args := descriptionArgs(repoDir, server.URL, "123", "45")
	if err := run(args, io.Discard); err != nil {
		t.Fatalf("first run() unexpected error: %v", err)
	}
	if !server.descriptionUpdated {
		t.Fatal("expected the first run to write the description")
	}

	firstDescription := server.description
	server.descriptionUpdated = false

	if err := run(args, io.Discard); err != nil {
		t.Fatalf("second run() unexpected error: %v", err)
	}

	if server.descriptionUpdated {
		t.Error("expected no write on the second run: the rendered report is identical")
	}
	if server.description != firstDescription {
		t.Errorf("description changed between identical runs:\n first: %q\nsecond: %q", firstDescription, server.description)
	}
}

func TestRunFailsOnUnterminatedDescriptionRegion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("GITHUB_ACTIONS", "")

	// The closing marker was deleted by hand: the bot cannot tell where its
	// region ends, and refuses to guess rather than risk deleting the text
	// below it.
	repoDir := setupOrchestrationRepo(t, testBaseLock, testHeadLock)
	server := newFakeGitLabServer(t, "123", "45", nil)
	server.description = "Intro.\n\n" + report.Marker + "\nstale\n\nCloses #12.\n"

	err := run(descriptionArgs(repoDir, server.URL, "123", "45"), io.Discard)
	if err == nil {
		t.Fatal("run() error = nil, want a failure on the unterminated region")
	}
	if !strings.Contains(err.Error(), "closing") {
		t.Errorf("run() error = %q, want it to explain the missing closing marker", err.Error())
	}
	if server.descriptionUpdated {
		t.Error("expected the description to be left untouched when the region cannot be delimited")
	}
}

func TestRunDescriptionModeStaysSilentWhenThereIsNothingToReport(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("GITHUB_ACTIONS", "")

	// No dependency change and no region yet: nothing was ever reported, so
	// the author's description is not touched to say so.
	repoDir := setupMultiFileOrchestrationRepo(t,
		map[string]string{"composer.lock": testBaseLock, "README.md": "base"},
		map[string]string{"composer.lock": testBaseLock, "README.md": "head"},
	)
	server := newFakeGitLabServer(t, "123", "45", nil)
	server.description = "Bumps the vendored packages.\n"

	if err := run(descriptionArgs(repoDir, server.URL, "123", "45"), io.Discard); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	if server.descriptionUpdated {
		t.Errorf("expected no description write, got %q", server.description)
	}
	if server.createCalled {
		t.Error("expected no Bot Comment either")
	}
}
