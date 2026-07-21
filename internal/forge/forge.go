// Package forge defines the seam between the orchestrator (cmd/dependency-diff-notes)
// and a specific Forge (see CONTEXT.md): the code-hosting platform — GitLab or
// GitHub — that hosts a Change Request and accepts a Bot Comment.
package forge

import "context"

// Comment is a single Bot Comment on a Change Request, as returned by a
// Forge's API (a GitLab note or a GitHub issue comment).
type Comment struct {
	ID   int
	Body string
}

// Client reads and writes the two Report Destinations (see CONTEXT.md) on a
// single Change Request: its comments, and its description. A Forge-specific
// implementation binds the Change Request's identity (project/repository and
// Change Request number) at construction, so these methods carry no
// Forge-specific identifiers.
//
// Every method is required of every Forge rather than split into an optional
// interface: both supported Forges can do all of it, and a run touches both
// destinations whichever one is in effect, since the bot cleans up the other
// (see docs/adr/0008-report-destination.md).
type Client interface {
	// ListComments returns all comments on the Change Request.
	ListComments(ctx context.Context) ([]Comment, error)

	// CreateComment posts a new comment with the given body on the Change Request.
	CreateComment(ctx context.Context, body string) (Comment, error)

	// UpdateComment replaces the body of an existing comment.
	UpdateComment(ctx context.Context, id int, body string) error

	// DeleteComment removes an existing comment, used to clear the Bot
	// Comment when the report has moved to the description.
	DeleteComment(ctx context.Context, id int) error

	// Description returns the Change Request's current description, the
	// document hosting the Description Region.
	Description(ctx context.Context) (string, error)

	// UpdateDescription replaces the Change Request's description.
	UpdateDescription(ctx context.Context, body string) error
}
