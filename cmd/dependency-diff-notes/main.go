package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jdecool/dependency-diff-notes/internal/composerlock"
	"github.com/jdecool/dependency-diff-notes/internal/config"
	"github.com/jdecool/dependency-diff-notes/internal/dependencydiff"
	"github.com/jdecool/dependency-diff-notes/internal/forge"
	"github.com/jdecool/dependency-diff-notes/internal/github"
	"github.com/jdecool/dependency-diff-notes/internal/gitlab"
	"github.com/jdecool/dependency-diff-notes/internal/gitref"
	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
	"github.com/jdecool/dependency-diff-notes/internal/npmlock"
	"github.com/jdecool/dependency-diff-notes/internal/report"
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
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run dispatches subcommands and returns an error instead of calling
// os.Exit directly, so it stays testable.
//
// It loads the bot's configuration, computes the Dependency Report (see
// CONTEXT.md) between the Change Request's target branch and its current
// commit, and creates or updates the Bot Comment on the Change Request
// accordingly.
func run(args []string) error {
	cfg, err := config.Load(args)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.InChangeRequestContext {
		fmt.Println("not running in a change request pipeline, nothing to do")
		return nil
	}

	diff, err := computeDiff(cfg)
	if err != nil {
		return err
	}

	return syncComment(context.Background(), cfg, diff)
}

// ecosystemSpec pairs one Ecosystem with how to locate and parse its
// Lockfile for this run.
type ecosystemSpec struct {
	ecosystem lockfile.Ecosystem
	lockPath  string
	parse     func([]byte) (lockfile.Lock, error)
}

// ecosystemSpecs returns the spec for every Ecosystem the bot knows how to
// read (see CONTEXT.md), bound to cfg's configured (or default) Lockfile
// path for each.
func ecosystemSpecs(cfg config.Config) []ecosystemSpec {
	return []ecosystemSpec{
		{lockfile.Composer, cfg.ComposerLockPath, composerlock.Parse},
		{lockfile.NPM, cfg.NPMLockPath, npmlock.Parse},
	}
}

// computeDiff resolves the merge-base between the Change Request's target
// branch and HEAD, and returns the Dependency Report: the Dependency
// Changes for every Ecosystem the bot knows how to read (see
// ecosystemSpecs), computed between the two Lockfile snapshots.
func computeDiff(cfg config.Config) (dependencydiff.Report, error) {
	base, err := gitref.MergeBase(cfg.RepoDir, "origin/"+cfg.TargetBranch, "HEAD")
	if err != nil {
		return dependencydiff.Report{}, fmt.Errorf(
			"resolve merge base (ensure the target branch %q is fetched, e.g. via GIT_DEPTH/git fetch in CI): %w",
			cfg.TargetBranch, err)
	}

	var report dependencydiff.Report

	for _, spec := range ecosystemSpecs(cfg) {
		baseLock, err := lockAtRef(cfg.RepoDir, base, spec.lockPath, spec.parse)
		if err != nil {
			return dependencydiff.Report{}, fmt.Errorf("read %s at merge base %q: %w", spec.lockPath, base, err)
		}

		headLock, err := lockAtRef(cfg.RepoDir, "HEAD", spec.lockPath, spec.parse)
		if err != nil {
			return dependencydiff.Report{}, fmt.Errorf("read %s at HEAD: %w", spec.lockPath, err)
		}

		report.Sections = append(report.Sections, dependencydiff.Diff(spec.ecosystem, baseLock, headLock))
	}

	return report, nil
}

// lockAtRef reads and parses a Lockfile at ref using parse, within the git
// repository rooted at repoDir. A missing file is treated as an empty Lock
// rather than an error: a Lockfile legitimately may not exist yet on one
// side (e.g. a project just adopting an Ecosystem, or one dropping it
// entirely). Any other read error, or a parse failure on a file that does
// exist, is a hard failure.
func lockAtRef(repoDir, ref, path string, parse func([]byte) (lockfile.Lock, error)) (lockfile.Lock, error) {
	data, err := gitref.FileAtRef(repoDir, ref, path)
	if err != nil {
		if errors.Is(err, gitref.ErrFileNotFound) {
			return lockfile.Lock{}, nil
		}
		return lockfile.Lock{}, fmt.Errorf("read %s at %s: %w", path, ref, err)
	}

	lock, err := parse(data)
	if err != nil {
		return lockfile.Lock{}, fmt.Errorf("parse %s at %s: %w", path, ref, err)
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
// reflect diff, per decideCommentAction, and prints a one-line confirmation
// of the action taken.
func syncComment(ctx context.Context, cfg config.Config, diff dependencydiff.Report) error {
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
		fmt.Println("created the Bot Comment with the detected dependency changes")

	case updateAction:
		if err := client.UpdateComment(ctx, existingComment.ID, body); err != nil {
			return fmt.Errorf("update comment: %w", err)
		}
		fmt.Println("updated the Bot Comment with the current dependency changes")

	case noAction:
		fmt.Println("no dependency changes to report")
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
