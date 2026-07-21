// Package github provides a minimal GitHub REST API client covering only
// what this bot needs: listing, creating, updating and deleting pull request
// (issue) comments, and reading and writing the pull request body. See docs/adr/0003-minimal-github-client.md for the rationale
// behind hand-writing this client instead of depending on an external SDK.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/forge"
)

// apiURL is the GitHub.com REST API base URL. GitHub Enterprise Server
// (which serves its API from a different host) is not supported.
const apiURL = "https://api.github.com"

// Client is a minimal GitHub REST API client covering only issue comments
// (GitHub's REST API models a pull request's comments as issue comments) for
// a single pull request, implementing forge.Client.
var _ forge.Client = (*Client)(nil)

type Client struct {
	baseURL    string // overridable in tests only; always apiURL in production
	token      string
	repository string // "owner/repo"
	prNumber   string
	httpClient *http.Client
}

// NewClient creates a Client authenticating with token via the Authorization
// header, bound to the pull request prNumber on repository ("owner/repo",
// e.g. from GITHUB_REPOSITORY). If httpClient is nil, http.DefaultClient is used.
func NewClient(token, repository, prNumber string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:    apiURL,
		token:      token,
		repository: repository,
		prNumber:   prNumber,
		httpClient: httpClient,
	}
}

// comment is the JSON shape of a GitHub issue comment.
type comment struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

// commentRequest is the JSON payload sent when creating or updating a comment.
type commentRequest struct {
	Body string `json:"body"`
}

// commentsPath builds the /repos/{owner}/{repo}/issues/{number}/comments
// path for the client's bound repository and pull request.
func (c *Client) commentsPath() string {
	return fmt.Sprintf("/repos/%s/issues/%s/comments", c.repository, c.prNumber)
}

// ListComments returns all comments on the bound pull request.
//
// This fetches a single page only: the bot only ever looks for one specific
// marker comment among a handful of comments, well within GitHub's default
// page size, so following Link pagination isn't needed for this scope.
func (c *Client) ListComments(ctx context.Context) ([]forge.Comment, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.commentsPath(), nil)
	if err != nil {
		return nil, fmt.Errorf("build list comments request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	var comments []comment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, fmt.Errorf("decode list comments response: %w", err)
	}

	result := make([]forge.Comment, len(comments))
	for i, cm := range comments {
		result[i] = forge.Comment{ID: cm.ID, Body: cm.Body}
	}

	return result, nil
}

// CreateComment posts a new comment with the given body on the bound pull request.
func (c *Client) CreateComment(ctx context.Context, body string) (forge.Comment, error) {
	payload, err := json.Marshal(commentRequest{Body: body})
	if err != nil {
		return forge.Comment{}, fmt.Errorf("encode create comment request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.commentsPath(), bytes.NewReader(payload))
	if err != nil {
		return forge.Comment{}, fmt.Errorf("build create comment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return forge.Comment{}, fmt.Errorf("create comment: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return forge.Comment{}, fmt.Errorf("create comment: %w", err)
	}

	var cm comment
	if err := json.NewDecoder(resp.Body).Decode(&cm); err != nil {
		return forge.Comment{}, fmt.Errorf("decode create comment response: %w", err)
	}

	return forge.Comment{ID: cm.ID, Body: cm.Body}, nil
}

// DeleteComment removes an existing comment, used to clear the Bot Comment
// when the report has moved to the description. Like updates, GitHub
// addresses comment deletions by repository, not by pull request number.
func (c *Client) DeleteComment(ctx context.Context, id int) error {
	path := fmt.Sprintf("/repos/%s/issues/comments/%d", c.repository, id)

	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("build delete comment request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}

	return nil
}

// pullPath builds the /repos/{owner}/{repo}/pulls/{number} path. The
// description lives on the pull request resource, unlike comments, which
// GitHub models as issue comments.
func (c *Client) pullPath() string {
	return fmt.Sprintf("/repos/%s/pulls/%s", c.repository, c.prNumber)
}

// pullRequest is the subset of a GitHub pull request the bot reads and
// writes. GitHub calls the description "body", the same word it uses for a
// comment's text.
type pullRequest struct {
	Body string `json:"body"`
}

// Description returns the bound pull request's current body.
func (c *Client) Description(ctx context.Context) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.pullPath(), nil)
	if err != nil {
		return "", fmt.Errorf("build get pull request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("get pull request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return "", fmt.Errorf("get pull request: %w", err)
	}

	var pr pullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return "", fmt.Errorf("decode pull request response: %w", err)
	}

	return pr.Body, nil
}

// UpdateDescription replaces the bound pull request's body.
func (c *Client) UpdateDescription(ctx context.Context, body string) error {
	payload, err := json.Marshal(pullRequest{Body: body})
	if err != nil {
		return fmt.Errorf("encode update pull request request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPatch, c.pullPath(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build update pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("update pull request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return fmt.Errorf("update pull request: %w", err)
	}

	return nil
}

// UpdateComment replaces the body of an existing comment. GitHub addresses
// comment updates by repository, not by issue/pull request number.
func (c *Client) UpdateComment(ctx context.Context, id int, body string) error {
	payload, err := json.Marshal(commentRequest{Body: body})
	if err != nil {
		return fmt.Errorf("encode update comment request: %w", err)
	}

	path := fmt.Sprintf("/repos/%s/issues/comments/%d", c.repository, id)
	req, err := c.newRequest(ctx, http.MethodPatch, path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build update comment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("update comment: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return fmt.Errorf("update comment: %w", err)
	}

	return nil
}

// newRequest builds an HTTP request against the GitHub API, setting the
// authentication and API-version headers shared by every call.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	return req, nil
}

// do executes req using the configured HTTP client.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

// checkStatus returns a descriptive error if resp did not succeed, including
// the status code and a snippet of the response body to help debug
// authentication or permission issues.
func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	const maxSnippet = 512

	snippet, err := io.ReadAll(io.LimitReader(resp.Body, maxSnippet))
	if err != nil {
		return fmt.Errorf("unexpected status %d (failed to read body: %v)", resp.StatusCode, err)
	}

	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
}
