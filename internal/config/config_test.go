package config

import (
	"strings"
	"testing"
)

// allEnvVars lists every environment variable Load ever reads, across both
// Forges, so each test case starts from a clean slate regardless of what's
// set in the actual test-runner environment.
var allEnvVars = []string{
	"GITHUB_ACTIONS",
	"CI_SERVER_URL",
	"CI_PROJECT_ID",
	"CI_MERGE_REQUEST_IID",
	"CI_MERGE_REQUEST_TARGET_BRANCH_NAME",
	"DEPENDENCY_DIFF_NOTES_TOKEN",
	"DEPENDENCY_DIFF_NOTES_COMPOSER_LOCK_PATH",
	"DEPENDENCY_DIFF_NOTES_NPM_LOCK_PATH",
	"DEPENDENCY_DIFF_NOTES_PNPM_LOCK_PATH",
	"GITHUB_REPOSITORY",
	"GITHUB_REF",
	"GITHUB_BASE_REF",
	"GITHUB_TOKEN",
}

func TestLoad_GitLab(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		want    Config
		wantErr string // non-empty substring expected in the error, "" means no error
	}{
		{
			name: "flags fully override env",
			args: []string{
				"--server-url", "https://flag.example.com",
				"--project-id", "flag-project",
				"--request-iid", "42",
				"--target-branch", "flag-branch",
				"--token", "flag-token",
				"--composer-lock-path", "flag-composer.lock",
				"--npm-lock-path", "flag-package-lock.json",
				"--pnpm-lock-path", "flag-pnpm-lock.yaml",
				"--repo-dir", "/flag/repo",
			},
			env: map[string]string{
				"CI_SERVER_URL":                            "https://env.example.com",
				"CI_PROJECT_ID":                            "env-project",
				"CI_MERGE_REQUEST_IID":                     "7",
				"CI_MERGE_REQUEST_TARGET_BRANCH_NAME":      "env-branch",
				"DEPENDENCY_DIFF_NOTES_TOKEN":              "env-token",
				"DEPENDENCY_DIFF_NOTES_COMPOSER_LOCK_PATH": "env-composer.lock",
				"DEPENDENCY_DIFF_NOTES_NPM_LOCK_PATH":      "env-package-lock.json",
				"DEPENDENCY_DIFF_NOTES_PNPM_LOCK_PATH":     "env-pnpm-lock.yaml",
			},
			want: Config{
				Forge:                  GitLab,
				GitLabServerURL:        "https://flag.example.com",
				ProjectID:              "flag-project",
				ChangeRequestIID:       "42",
				TargetBranch:           "flag-branch",
				Token:                  "flag-token",
				ComposerLockPath:       "flag-composer.lock",
				NPMLockPath:            "flag-package-lock.json",
				PnpmLockPath:           "flag-pnpm-lock.yaml",
				RepoDir:                "/flag/repo",
				InChangeRequestContext: true,
			},
		},
		{
			name: "env-only resolution when no flags given",
			args: []string{},
			env: map[string]string{
				"CI_SERVER_URL":                            "https://env.example.com",
				"CI_PROJECT_ID":                            "env-project",
				"CI_MERGE_REQUEST_IID":                     "7",
				"CI_MERGE_REQUEST_TARGET_BRANCH_NAME":      "env-branch",
				"DEPENDENCY_DIFF_NOTES_TOKEN":              "env-token",
				"DEPENDENCY_DIFF_NOTES_COMPOSER_LOCK_PATH": "env-composer.lock",
				"DEPENDENCY_DIFF_NOTES_NPM_LOCK_PATH":      "env-package-lock.json",
				"DEPENDENCY_DIFF_NOTES_PNPM_LOCK_PATH":     "env-pnpm-lock.yaml",
			},
			want: Config{
				Forge:                  GitLab,
				GitLabServerURL:        "https://env.example.com",
				ProjectID:              "env-project",
				ChangeRequestIID:       "7",
				TargetBranch:           "env-branch",
				Token:                  "env-token",
				ComposerLockPath:       "env-composer.lock",
				NPMLockPath:            "env-package-lock.json",
				PnpmLockPath:           "env-pnpm-lock.yaml",
				RepoDir:                defaultRepoDir,
				InChangeRequestContext: true,
			},
		},
		{
			name: "no request IID anywhere means not in change request context and no error",
			args: []string{},
			env:  map[string]string{},
			want: Config{
				Forge:                  GitLab,
				ComposerLockPath:       defaultComposerLockPath,
				NPMLockPath:            defaultNPMLockPath,
				PnpmLockPath:           defaultPnpmLockPath,
				RepoDir:                defaultRepoDir,
				InChangeRequestContext: false,
			},
		},
		{
			name: "request IID present but token missing is an error",
			args: []string{},
			env: map[string]string{
				"CI_SERVER_URL":                       "https://env.example.com",
				"CI_PROJECT_ID":                       "env-project",
				"CI_MERGE_REQUEST_IID":                "7",
				"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "env-branch",
			},
			wantErr: "token",
		},
		{
			name: "ComposerLockPath and RepoDir default correctly when unset",
			args: []string{},
			env:  map[string]string{},
			want: Config{
				Forge:                  GitLab,
				ComposerLockPath:       defaultComposerLockPath,
				NPMLockPath:            defaultNPMLockPath,
				PnpmLockPath:           defaultPnpmLockPath,
				RepoDir:                defaultRepoDir,
				InChangeRequestContext: false,
			},
		},
		{
			name: "request IID present but server-url and project-id missing lists both",
			args: []string{
				"--request-iid", "1",
				"--target-branch", "main",
				"--token", "tok",
			},
			env:     map[string]string{},
			wantErr: "server-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range allEnvVars {
				t.Setenv(name, tt.env[name])
			}

			got, err := Load(tt.args)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoad_GitHub(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		want    Config
		wantErr string
	}{
		{
			name: "pull_request event resolves from GitHub Actions env",
			args: []string{},
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "acme/widget",
				"GITHUB_REF":        "refs/pull/57/merge",
				"GITHUB_BASE_REF":   "main",
				"GITHUB_TOKEN":      "gh-token",
			},
			want: Config{
				Forge:                  GitHub,
				ProjectID:              "acme/widget",
				ChangeRequestIID:       "57",
				TargetBranch:           "main",
				Token:                  "gh-token",
				ComposerLockPath:       defaultComposerLockPath,
				NPMLockPath:            defaultNPMLockPath,
				PnpmLockPath:           defaultPnpmLockPath,
				RepoDir:                defaultRepoDir,
				InChangeRequestContext: true,
			},
		},
		{
			name: "flags override GitHub env",
			args: []string{
				"--project-id", "flag/repo",
				"--request-iid", "99",
				"--target-branch", "flag-branch",
				"--token", "flag-token",
			},
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "acme/widget",
				"GITHUB_REF":        "refs/pull/57/merge",
				"GITHUB_BASE_REF":   "main",
				"GITHUB_TOKEN":      "gh-token",
			},
			want: Config{
				Forge:                  GitHub,
				ProjectID:              "flag/repo",
				ChangeRequestIID:       "99",
				TargetBranch:           "flag-branch",
				Token:                  "flag-token",
				ComposerLockPath:       defaultComposerLockPath,
				NPMLockPath:            defaultNPMLockPath,
				PnpmLockPath:           defaultPnpmLockPath,
				RepoDir:                defaultRepoDir,
				InChangeRequestContext: true,
			},
		},
		{
			name: "push event ref means not in change request context and no error",
			args: []string{},
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "acme/widget",
				"GITHUB_REF":        "refs/heads/main",
			},
			want: Config{
				Forge:                  GitHub,
				ProjectID:              "acme/widget",
				ComposerLockPath:       defaultComposerLockPath,
				NPMLockPath:            defaultNPMLockPath,
				PnpmLockPath:           defaultPnpmLockPath,
				RepoDir:                defaultRepoDir,
				InChangeRequestContext: false,
			},
		},
		{
			name: "pull request event but token missing is an error",
			args: []string{},
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "acme/widget",
				"GITHUB_REF":        "refs/pull/57/merge",
				"GITHUB_BASE_REF":   "main",
			},
			wantErr: "token (or GITHUB_TOKEN)",
		},
		{
			name: "server-url is never required on GitHub",
			args: []string{},
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "acme/widget",
				"GITHUB_REF":        "refs/pull/57/merge",
				"GITHUB_BASE_REF":   "main",
				"GITHUB_TOKEN":      "gh-token",
			},
			want: Config{
				Forge:                  GitHub,
				ProjectID:              "acme/widget",
				ChangeRequestIID:       "57",
				TargetBranch:           "main",
				Token:                  "gh-token",
				ComposerLockPath:       defaultComposerLockPath,
				NPMLockPath:            defaultNPMLockPath,
				PnpmLockPath:           defaultPnpmLockPath,
				RepoDir:                defaultRepoDir,
				InChangeRequestContext: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range allEnvVars {
				t.Setenv(name, tt.env[name])
			}

			got, err := Load(tt.args)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name          string
		githubActions string
		want          Forge
	}{
		{name: "GITHUB_ACTIONS=true means GitHub", githubActions: "true", want: GitHub},
		{name: "GITHUB_ACTIONS unset means GitLab", githubActions: "", want: GitLab},
		{name: "GITHUB_ACTIONS=false means GitLab", githubActions: "false", want: GitLab},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_ACTIONS", tt.githubActions)
			if got := Detect(); got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseGitHubPRNumber(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{name: "pull request ref", ref: "refs/pull/57/merge", want: "57"},
		{name: "branch ref", ref: "refs/heads/main", want: ""},
		{name: "tag ref", ref: "refs/tags/v1.0.0", want: ""},
		{name: "empty ref", ref: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseGitHubPRNumber(tt.ref); got != tt.want {
				t.Errorf("parseGitHubPRNumber(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}
