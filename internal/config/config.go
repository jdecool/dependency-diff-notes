// Package config resolves the bot's configuration from CLI flags and the
// active Forge's predefined CI environment variables for a single invocation.
package config

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Forge identifies which code-hosting platform (see CONTEXT.md) the bot is
// running against for this invocation.
type Forge int

const (
	// GitLab is the default Forge, detected in the absence of GitHub
	// Actions' own environment variables.
	GitLab Forge = iota
	GitHub
)

// String returns the human-readable Forge name, used in flag descriptions
// and error messages.
func (f Forge) String() string {
	if f == GitHub {
		return "GitHub"
	}
	return "GitLab"
}

// Config holds everything the bot needs to run for one invocation.
type Config struct {
	Forge Forge

	GitLabServerURL        string // GitLab only; empty on GitHub (api.github.com is hardcoded)
	ProjectID              string // GitLab project ID, or GitHub "owner/repo"
	ChangeRequestIID       string // Merge Request IID (GitLab) or Pull Request number (GitHub)
	TargetBranch           string
	Token                  string
	ComposerLockPath       string // path to composer.lock (Composer Ecosystem, see CONTEXT.md)
	NPMLockPath            string // path to package-lock.json (npm Ecosystem, see CONTEXT.md)
	PnpmLockPath           string // path to pnpm-lock.yaml (pnpm Ecosystem, see CONTEXT.md)
	RepoDir                string
	InChangeRequestContext bool // true iff ChangeRequestIID resolved to a non-empty value
}

const (
	defaultComposerLockPath = "composer.lock"
	defaultNPMLockPath      = "package-lock.json"
	defaultPnpmLockPath     = "pnpm-lock.yaml"
	defaultRepoDir          = "."
)

// githubRefPullPattern matches the ref format GitHub Actions sets for
// pull_request (and pull_request_target) events: "refs/pull/<number>/merge".
// Any other ref (e.g. "refs/heads/main" on a push event) means the run isn't
// tied to a pull request.
var githubRefPullPattern = regexp.MustCompile(`^refs/pull/(\d+)/merge$`)

// parseGitHubPRNumber extracts the pull request number from a GITHUB_REF
// value, or returns "" if ref isn't a pull_request-event ref.
func parseGitHubPRNumber(ref string) string {
	m := githubRefPullPattern.FindStringSubmatch(ref)
	if m == nil {
		return ""
	}
	return m[1]
}

// Detect reports which Forge the bot is running under, based on the
// environment variables each CI system sets on its own runners: GitHub
// Actions always sets GITHUB_ACTIONS=true. Anything else defaults to GitLab.
func Detect() Forge {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return GitHub
	}
	return GitLab
}

// Load resolves configuration from CLI args (e.g. os.Args[1:]) and environment
// variables (read via os.Getenv), with flags taking precedence over the
// corresponding environment variable when both are set. Which environment
// variables are consulted depends on the Forge detected by Detect.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("dependency-diff-notes", flag.ContinueOnError)

	serverURL := fs.String("server-url", "", "GitLab server URL (default: $CI_SERVER_URL; unused on GitHub)")
	projectID := fs.String("project-id", "", "GitLab project ID or GitHub repository (default: $CI_PROJECT_ID or $GITHUB_REPOSITORY)")
	requestIID := fs.String("request-iid", "", "Change Request IID/number (default: $CI_MERGE_REQUEST_IID, or parsed from $GITHUB_REF)")
	targetBranch := fs.String("target-branch", "", "Change Request target branch (default: $CI_MERGE_REQUEST_TARGET_BRANCH_NAME or $GITHUB_BASE_REF)")
	token := fs.String("token", "", "Forge API token (default: $DEPENDENCY_DIFF_NOTES_TOKEN or $GITHUB_TOKEN)")
	composerLockPath := fs.String("composer-lock-path", "", "Path to composer.lock (default: $DEPENDENCY_DIFF_NOTES_COMPOSER_LOCK_PATH, or \"composer.lock\")")
	npmLockPath := fs.String("npm-lock-path", "", "Path to package-lock.json (default: $DEPENDENCY_DIFF_NOTES_NPM_LOCK_PATH, or \"package-lock.json\")")
	pnpmLockPath := fs.String("pnpm-lock-path", "", "Path to pnpm-lock.yaml (default: $DEPENDENCY_DIFF_NOTES_PNPM_LOCK_PATH, or \"pnpm-lock.yaml\")")
	repoDir := fs.String("repo-dir", "", "Path to the repository checkout (default: \".\")")

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	forge := Detect()

	cfg := Config{
		Forge:            forge,
		ComposerLockPath: resolve(*composerLockPath, "DEPENDENCY_DIFF_NOTES_COMPOSER_LOCK_PATH", defaultComposerLockPath),
		NPMLockPath:      resolve(*npmLockPath, "DEPENDENCY_DIFF_NOTES_NPM_LOCK_PATH", defaultNPMLockPath),
		PnpmLockPath:     resolve(*pnpmLockPath, "DEPENDENCY_DIFF_NOTES_PNPM_LOCK_PATH", defaultPnpmLockPath),
		RepoDir:          resolveNoEnv(*repoDir, defaultRepoDir),
	}

	switch forge {
	case GitHub:
		cfg.ProjectID = resolve(*projectID, "GITHUB_REPOSITORY", "")
		cfg.ChangeRequestIID = resolve(*requestIID, "", parseGitHubPRNumber(os.Getenv("GITHUB_REF")))
		cfg.TargetBranch = resolve(*targetBranch, "GITHUB_BASE_REF", "")
		cfg.Token = resolve(*token, "GITHUB_TOKEN", "")
	default:
		cfg.GitLabServerURL = resolve(*serverURL, "CI_SERVER_URL", "")
		cfg.ProjectID = resolve(*projectID, "CI_PROJECT_ID", "")
		cfg.ChangeRequestIID = resolve(*requestIID, "CI_MERGE_REQUEST_IID", "")
		cfg.TargetBranch = resolve(*targetBranch, "CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "")
		cfg.Token = resolve(*token, "DEPENDENCY_DIFF_NOTES_TOKEN", "")
	}

	cfg.InChangeRequestContext = cfg.ChangeRequestIID != ""

	if cfg.InChangeRequestContext {
		if missing := missingSettings(cfg); len(missing) > 0 {
			return Config{}, fmt.Errorf("resolve config: missing required change request settings: %s", strings.Join(missing, ", "))
		}
	}

	return cfg, nil
}

// missingSettings returns the flags/env vars still unresolved that are
// required once the bot has detected it's running in a Change Request
// context, describing each in terms of the active Forge's own env vars.
func missingSettings(cfg Config) []string {
	var missing []string

	if cfg.Forge == GitLab && cfg.GitLabServerURL == "" {
		missing = append(missing, "server-url (or CI_SERVER_URL)")
	}
	if cfg.ProjectID == "" {
		if cfg.Forge == GitHub {
			missing = append(missing, "project-id (or GITHUB_REPOSITORY)")
		} else {
			missing = append(missing, "project-id (or CI_PROJECT_ID)")
		}
	}
	if cfg.TargetBranch == "" {
		if cfg.Forge == GitHub {
			missing = append(missing, "target-branch (or GITHUB_BASE_REF)")
		} else {
			missing = append(missing, "target-branch (or CI_MERGE_REQUEST_TARGET_BRANCH_NAME)")
		}
	}
	if cfg.Token == "" {
		if cfg.Forge == GitHub {
			missing = append(missing, "token (or GITHUB_TOKEN)")
		} else {
			missing = append(missing, "token (or DEPENDENCY_DIFF_NOTES_TOKEN)")
		}
	}

	return missing
}

// resolve returns the flag value if set, otherwise the environment variable
// named envVar if set, otherwise def. envVar may be "" to skip the
// environment lookup entirely (the caller has already computed its own
// fallback, e.g. by parsing another variable).
func resolve(flagValue, envVar, def string) string {
	if flagValue != "" {
		return flagValue
	}
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return v
		}
	}
	return def
}

// resolveNoEnv returns the flag value if set, otherwise def. Used for fields
// without an environment variable fallback.
func resolveNoEnv(flagValue, def string) string {
	if flagValue != "" {
		return flagValue
	}
	return def
}
