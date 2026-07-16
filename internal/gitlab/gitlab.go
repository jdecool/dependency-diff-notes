// Package gitlab provides a minimal GitLab REST API v4 client covering only
// what this bot needs: listing, creating, and updating merge request notes.
// See docs/adr/0001-minimal-gitlab-client.md for the rationale behind
// hand-writing this client instead of depending on an external SDK.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/forge"
)

// Client is a minimal GitLab REST API v4 client covering only merge request
// notes for a single merge request, implementing forge.Client.
var _ forge.Client = (*Client)(nil)

type Client struct {
	baseURL    string
	token      string
	projectID  string
	mrIID      string
	httpClient *http.Client
}

// NewClient creates a Client for the GitLab instance at baseURL (e.g. "https://gitlab.com"),
// authenticating with token via the PRIVATE-TOKEN header, bound to the merge
// request mrIID on projectID (a numeric ID or a namespaced path, e.g.
// "group/project"). If httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL, token, projectID, mrIID string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		projectID:  projectID,
		mrIID:      mrIID,
		httpClient: httpClient,
	}
}

// note is the JSON shape of a GitLab merge request note.
type note struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

// noteRequest is the JSON payload sent when creating or updating a note.
type noteRequest struct {
	Body string `json:"body"`
}

// notesPath builds the /merge_requests/{mrIID}/notes path for the client's
// bound project and merge request.
func (c *Client) notesPath() string {
	return fmt.Sprintf("/api/v4/projects/%s/merge_requests/%s/notes", url.PathEscape(c.projectID), url.PathEscape(c.mrIID))
}

// ListComments returns all notes on the bound merge request.
//
// This fetches a single page only: the bot only ever looks for one specific
// marker note among a handful of comments, well within GitLab's default page
// size, so following X-Next-Page pagination isn't needed for this scope.
func (c *Client) ListComments(ctx context.Context) ([]forge.Comment, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.notesPath(), nil)
	if err != nil {
		return nil, fmt.Errorf("build list notes request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}

	var notes []note
	if err := json.NewDecoder(resp.Body).Decode(&notes); err != nil {
		return nil, fmt.Errorf("decode list notes response: %w", err)
	}

	comments := make([]forge.Comment, len(notes))
	for i, n := range notes {
		comments[i] = forge.Comment{ID: n.ID, Body: n.Body}
	}

	return comments, nil
}

// CreateComment posts a new note with the given body on the bound merge request.
func (c *Client) CreateComment(ctx context.Context, body string) (forge.Comment, error) {
	payload, err := json.Marshal(noteRequest{Body: body})
	if err != nil {
		return forge.Comment{}, fmt.Errorf("encode create note request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.notesPath(), bytes.NewReader(payload))
	if err != nil {
		return forge.Comment{}, fmt.Errorf("build create note request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return forge.Comment{}, fmt.Errorf("create note: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return forge.Comment{}, fmt.Errorf("create note: %w", err)
	}

	var n note
	if err := json.NewDecoder(resp.Body).Decode(&n); err != nil {
		return forge.Comment{}, fmt.Errorf("decode create note response: %w", err)
	}

	return forge.Comment{ID: n.ID, Body: n.Body}, nil
}

// UpdateComment replaces the body of an existing note.
func (c *Client) UpdateComment(ctx context.Context, id int, body string) error {
	payload, err := json.Marshal(noteRequest{Body: body})
	if err != nil {
		return fmt.Errorf("encode update note request: %w", err)
	}

	path := fmt.Sprintf("%s/%d", c.notesPath(), id)
	req, err := c.newRequest(ctx, http.MethodPut, path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build update note request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return fmt.Errorf("update note: %w", err)
	}

	return nil
}

// newRequest builds an HTTP request against the client's base URL, setting
// the authentication header shared by every call.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

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
