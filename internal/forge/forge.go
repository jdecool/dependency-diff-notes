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

// Client lists, creates, and updates Bot Comments on a single Change
// Request. A Forge-specific implementation binds the Change Request's
// identity (project/repository and Change Request number) at construction,
// so these methods carry no Forge-specific identifiers.
type Client interface {
	// ListComments returns all comments on the Change Request.
	ListComments(ctx context.Context) ([]Comment, error)

	// CreateComment posts a new comment with the given body on the Change Request.
	CreateComment(ctx context.Context, body string) (Comment, error)

	// UpdateComment replaces the body of an existing comment.
	UpdateComment(ctx context.Context, id int, body string) error
}
