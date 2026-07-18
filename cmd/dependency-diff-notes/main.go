package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/composerlock"
	"github.com/jdecool/dependency-diff-notes/internal/config"
	"github.com/jdecool/dependency-diff-notes/internal/dependencydiff"
	"github.com/jdecool/dependency-diff-notes/internal/forge"
	"github.com/jdecool/dependency-diff-notes/internal/github"
	"github.com/jdecool/dependency-diff-notes/internal/gitlab"
	"github.com/jdecool/dependency-diff-notes/internal/gitref"
	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
	"github.com/jdecool/dependency-diff-notes/internal/npmlock"
	"github.com/jdecool/dependency-diff-notes/internal/pnpmlock"
	"github.com/jdecool/dependency-diff-notes/internal/report"
	"github.com/jdecool/dependency-diff-notes/internal/yarnlock"
)

// banner is the ASCII-art title printed on startup. The art embeds a
// backtick (in the `_` sequence on the third line), so the raw string
// literal is broken with a concatenated "`" to include it.
const banner = `
 ____                            _
|  _ \  ___ _ __   ___ _ __   __| | ___ _ __   ___ _   _
| | | |/ _ \ '_ \ / _ \ '_ \ / _` + "`" + `|/ _ \ '_ \ / __| | | |
| |_| |  __/ |_) |  __/ | | | (_| |  __/ | | | (__| |_| |
|____/ \___| .__/ \___|_| |_|\__,_|\___|_| |_|\___|\__, |
           |_|                                     |___/
   ____  _  __  __   _   _       _
  |  _ \(_)/ _|/ _| | \ | | ___ | |_ ___  ___
  | | | | | |_| |_  |  \| |/ _ \| __/ _ \/ __|
  | |_| | |  _|  _| | |\  | (_) | ||  __/\__ \
  |____/|_|_| |_|   |_| \_|\___/ \__\___||___/

`

func main() {
	fmt.Fprint(os.Stderr, banner)
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run loads the bot's configuration and dispatches to one of the two run
// modes, returning an error instead of calling os.Exit directly so it stays
// testable. All human-facing output is written to out.
//
// The mode is auto-detected, never selected by a subcommand or flag:
//   - In a Change Request context (a Change Request IID resolved), it computes
//     the Dependency Report between the Change Request's target branch and its
//     current commit and creates or updates the Bot Comment (see CONTEXT.md).
//   - Otherwise, if a base branch was given (--target-branch), it runs a Local
//     Comparison (see CONTEXT.md): it computes the same Dependency Report
//     between that branch and the Source, and prints it to out instead of
//     posting anywhere.
//   - Otherwise there is nothing to do.
func run(args []string, out io.Writer) error {
	cfg, err := config.Load(args)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	switch {
	case cfg.InChangeRequestContext:
		diff, err := computeDiff(cfg)
		if err != nil {
			return err
		}
		return syncComment(context.Background(), cfg, diff, out)

	case cfg.TargetBranch != "":
		diff, err := computeDiff(cfg)
		if err != nil {
			return err
		}
		fmt.Fprint(out, report.RenderText(diff))
		return nil

	default:
		fmt.Fprintln(out, "not running in a change request pipeline; pass --target-branch to compare locally")
		return nil
	}
}

// ecosystemSpec pairs one Ecosystem with how to locate and parse its
// Lockfile for this run.
type ecosystemSpec struct {
	ecosystem lockfile.Ecosystem
	lockPath  string
	parse     func([]byte) (lockfile.Lock, error)
	// jsFamily marks Ecosystems whose Lockfiles are mutually exclusive
	// (see CONTEXT.md: Lockfile) — a project uses at most one JavaScript
	// package manager at a time, so more than one of these Lockfiles
	// present at the same ref is a conflict, not a legitimate
	// multi-Ecosystem Change Request the way Composer + npm is.
	jsFamily bool
}

// ecosystemSpecs returns the spec for every Ecosystem the bot knows how to
// read (see CONTEXT.md), bound to cfg's configured (or default) Lockfile path
// for each, restricted to the Considered Ecosystems (see CONTEXT.md): when the
// operator declares an allowlist, the excluded Ecosystems are dropped for the
// whole run, which is also what defuses the JavaScript Lockfile conflict when
// the allowlist keeps at most one JavaScript Ecosystem.
func ecosystemSpecs(cfg config.Config) []ecosystemSpec {
	all := []ecosystemSpec{
		{lockfile.Composer, cfg.ComposerLockPath, composerlock.Parse, false},
		{lockfile.NPM, cfg.NPMLockPath, npmlock.Parse, true},
		{lockfile.Pnpm, cfg.PnpmLockPath, pnpmlock.Parse, true},
		{lockfile.Yarn, cfg.YarnLockPath, yarnlock.Parse, true},
	}

	var specs []ecosystemSpec
	for _, spec := range all {
		if cfg.ConsidersEcosystem(spec.ecosystem) {
			specs = append(specs, spec)
		}
	}

	return specs
}

// fileReader reads a repository-relative path from one side of a comparison,
// returning an error wrapping gitref.ErrFileNotFound when the path is absent
// there. Its two implementations read from a git ref (refReader) or from the
// on-disk working tree (workTreeReader).
type fileReader func(path string) ([]byte, error)

// endpoint is one side of the Dependency Report comparison: a way to read a
// Lockfile there, plus a human-readable label used in error messages.
type endpoint struct {
	read  fileReader
	label string
}

// refReader reads paths as they existed at ref within the repository rooted
// at repoDir (via git show).
func refReader(repoDir, ref string) fileReader {
	return func(path string) ([]byte, error) {
		return gitref.FileAtRef(repoDir, ref, path)
	}
}

// workTreeReader reads paths from the on-disk working tree rooted at repoDir,
// including uncommitted changes. A missing file is reported as
// gitref.ErrFileNotFound so callers can treat both endpoints uniformly.
func workTreeReader(repoDir string) fileReader {
	return func(path string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(repoDir, path))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, gitref.ErrFileNotFound
			}
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		return data, nil
	}
}

// resolveEndpoints determines the base and head endpoints of the comparison
// from cfg, resolving the merge-base once.
//
// The base is always the merge-base of the target branch and the head
// revision. In a Change Request context the target branch is taken as
// "origin/<branch>" (CI checkouts have the remote-tracking ref, not
// necessarily a local branch); in a Local Comparison it is used literally, so
// a local branch, tag, or SHA all work (see CONTEXT.md, and
// docs/adr/0006-local-comparison-mode.md).
//
// The head is HEAD in a Change Request context. In a Local Comparison it is
// the Source: the on-disk working tree by default (cfg.Source == ""), or the
// given ref otherwise.
func resolveEndpoints(cfg config.Config) (base, head endpoint, err error) {
	headRev := "HEAD"
	if cfg.Source != "" {
		headRev = cfg.Source
	}

	targetBranch := cfg.TargetBranch
	if cfg.InChangeRequestContext {
		targetBranch = "origin/" + cfg.TargetBranch
	}

	baseRef, err := gitref.MergeBase(cfg.RepoDir, targetBranch, headRev)
	if err != nil {
		return endpoint{}, endpoint{}, fmt.Errorf(
			"resolve merge base of %q and %q (ensure %q is fetched, e.g. via git fetch): %w",
			targetBranch, headRev, targetBranch, err)
	}

	base = endpoint{
		read:  refReader(cfg.RepoDir, baseRef),
		label: fmt.Sprintf("merge base %q", baseRef),
	}

	if !cfg.InChangeRequestContext && cfg.Source == "" {
		head = endpoint{read: workTreeReader(cfg.RepoDir), label: "working tree"}
	} else {
		head = endpoint{read: refReader(cfg.RepoDir, headRev), label: headRev}
	}

	return base, head, nil
}

// computeDiff returns the Dependency Report: the Dependency Changes for every
// Ecosystem the bot knows how to read (see ecosystemSpecs), computed between
// the base and head endpoints resolved from cfg.
func computeDiff(cfg config.Config) (dependencydiff.Report, error) {
	base, head, err := resolveEndpoints(cfg)
	if err != nil {
		return dependencydiff.Report{}, err
	}

	specs := ecosystemSpecs(cfg)

	for _, ep := range []endpoint{base, head} {
		if err := checkJSFamilyConflict(ep, specs); err != nil {
			return dependencydiff.Report{}, err
		}
	}

	var report dependencydiff.Report

	for _, spec := range specs {
		baseLock, err := lockAt(base.read, spec.lockPath, spec.parse)
		if err != nil {
			return dependencydiff.Report{}, fmt.Errorf("%s: %w", base.label, err)
		}

		headLock, err := lockAt(head.read, spec.lockPath, spec.parse)
		if err != nil {
			return dependencydiff.Report{}, fmt.Errorf("%s: %w", head.label, err)
		}

		report.Sections = append(report.Sections, dependencydiff.Diff(spec.ecosystem, baseLock, headLock))
	}

	return report, nil
}

// checkJSFamilyConflict returns an error if more than one jsFamily
// Ecosystem's Lockfile exists at the endpoint (see CONTEXT.md: Lockfile) —
// e.g. both package-lock.json and pnpm-lock.yaml present at once. The bot
// refuses to guess which package manager is actually in use rather than
// silently reporting both or picking one. This check runs per endpoint
// (independently for the merge-base and the head), not across the two: a
// migration from one JavaScript package manager to another is not a conflict,
// since the two Lockfiles never coexist at the same endpoint.
func checkJSFamilyConflict(ep endpoint, specs []ecosystemSpec) error {
	var present []string

	for _, spec := range specs {
		if !spec.jsFamily {
			continue
		}

		if _, err := ep.read(spec.lockPath); err != nil {
			if errors.Is(err, gitref.ErrFileNotFound) {
				continue
			}
			return fmt.Errorf("check %s at %s: %w", spec.lockPath, ep.label, err)
		}

		present = append(present, fmt.Sprintf("%s (%s)", spec.lockPath, spec.ecosystem))
	}

	if len(present) > 1 {
		return fmt.Errorf(
			"conflicting JavaScript Lockfiles at %s: %s — a project uses at most one JavaScript package manager at a time",
			ep.label, strings.Join(present, ", "))
	}

	return nil
}

// lockAt reads and parses a Lockfile using read (one endpoint's fileReader)
// and parse. A missing file is treated as an empty Lock rather than an error:
// a Lockfile legitimately may not exist yet on one side (e.g. a project just
// adopting an Ecosystem, or one dropping it entirely). Any other read error,
// or a parse failure on a file that does exist, is a hard failure.
func lockAt(read fileReader, path string, parse func([]byte) (lockfile.Lock, error)) (lockfile.Lock, error) {
	data, err := read(path)
	if err != nil {
		if errors.Is(err, gitref.ErrFileNotFound) {
			return lockfile.Lock{}, nil
		}
		return lockfile.Lock{}, err
	}

	lock, err := parse(data)
	if err != nil {
		return lockfile.Lock{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return lock, nil
}

// newForgeClient builds the forge.Client for cfg's detected Forge, bound to
// the Change Request identified in cfg.
func newForgeClient(cfg config.Config) forge.Client {
	switch cfg.Forge {
	case config.GitHub:
		return github.NewClient(cfg.Token, cfg.ProjectID, cfg.ChangeRequestIID, nil)
	default:
		return gitlab.NewClient(cfg.GitLabServerURL, cfg.Token, cfg.ProjectID, cfg.ChangeRequestIID, nil)
	}
}

// syncComment creates or updates the Bot Comment on the Change Request to
// reflect diff, per decideCommentAction, and writes a one-line confirmation
// of the action taken to out.
func syncComment(ctx context.Context, cfg config.Config, diff dependencydiff.Report, out io.Writer) error {
	client := newForgeClient(cfg)

	comments, err := client.ListComments(ctx)
	if err != nil {
		return fmt.Errorf("list comments: %w", err)
	}

	existingComment, found := findBotComment(comments)
	body := report.Render(diff)

	switch decideCommentAction(diff.IsEmpty(), found) {
	case createAction:
		if _, err := client.CreateComment(ctx, body); err != nil {
			return fmt.Errorf("create comment: %w", err)
		}
		fmt.Fprintln(out, "created the Bot Comment with the detected dependency changes")

	case updateAction:
		if err := client.UpdateComment(ctx, existingComment.ID, body); err != nil {
			return fmt.Errorf("update comment: %w", err)
		}
		fmt.Fprintln(out, "updated the Bot Comment with the current dependency changes")

	case noAction:
		fmt.Fprintln(out, "no dependency changes to report")
	}

	return nil
}

// findBotComment returns the first comment among comments identified as the
// bot's existing Bot Comment (see CONTEXT.md: the single comment the bot
// maintains, identified by a hidden marker), and whether one was found.
func findBotComment(comments []forge.Comment) (forge.Comment, bool) {
	for _, c := range comments {
		if report.HasMarker(c.Body) {
			return c, true
		}
	}

	return forge.Comment{}, false
}

// commentAction is the action to take on the Bot Comment for a single run.
type commentAction int

const (
	noAction commentAction = iota
	createAction
	updateAction
)

// decideCommentAction decides what to do with the Bot Comment given whether
// the current Dependency Report is empty and whether a Bot Comment already
// exists on the Change Request.
//
// Rules:
//   - no existing Bot Comment, empty diff: there was never anything to
//     report, so don't create one and add noise.
//   - no existing Bot Comment, non-empty diff: create it.
//   - existing Bot Comment, regardless of whether the diff is empty: keep it
//     in sync, including updating it to say "no changes" if a previously
//     reported change was reverted.
func decideCommentAction(diffIsEmpty, existingCommentFound bool) commentAction {
	switch {
	case existingCommentFound:
		return updateAction
	case diffIsEmpty:
		return noAction
	default:
		return createAction
	}
}
