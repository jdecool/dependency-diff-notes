package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jdecool/dependency-diff-notes/internal/forge"
)

const (
	testToken      = "s3cr3t-token"
	testRepository = "owner/repo"
	testPRNumber   = "42"
)

// newTestClient creates a Client authenticating against server instead of
// the real GitHub API.
func newTestClient(server *httptest.Server) *Client {
	c := NewClient(testToken, testRepository, testPRNumber, nil)
	c.baseURL = server.URL
	return c
}

func TestListComments(t *testing.T) {
	tests := []struct {
		name         string
		handler      http.HandlerFunc
		wantComments []forge.Comment
		wantErr      bool
		wantErrSub   string
	}{
		{
			name: "success with multiple comments",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if want := "/repos/owner/repo/issues/42/comments"; r.URL.EscapedPath() != want {
					t.Errorf("path = %s, want %s", r.URL.EscapedPath(), want)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
					t.Errorf("Authorization = %q, want %q", got, "Bearer "+testToken)
				}
				if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
					t.Errorf("Accept = %q, want application/vnd.github+json", got)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[{"id":1,"body":"first comment"},{"id":2,"body":"second comment"}]`))
			},
			wantComments: []forge.Comment{
				{ID: 1, Body: "first comment"},
				{ID: 2, Body: "second comment"},
			},
		},
		{
			name: "unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			},
			wantErr:    true,
			wantErrSub: "401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := newTestClient(server)
			comments, err := client.ListComments(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("ListComments() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("ListComments() error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("ListComments() unexpected error: %v", err)
			}

			if len(comments) != len(tt.wantComments) {
				t.Fatalf("ListComments() returned %d comments, want %d", len(comments), len(tt.wantComments))
			}
			for i, c := range comments {
				if c != tt.wantComments[i] {
					t.Errorf("comment[%d] = %+v, want %+v", i, c, tt.wantComments[i])
				}
			}
		})
	}
}

func TestCreateComment(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		handler     http.HandlerFunc
		wantComment forge.Comment
		wantErr     bool
		wantErrSub  string
	}{
		{
			name: "success",
			body: "dependencies updated",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if want := "/repos/owner/repo/issues/42/comments"; r.URL.EscapedPath() != want {
					t.Errorf("path = %s, want %s", r.URL.EscapedPath(), want)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
					t.Errorf("Authorization = %q, want %q", got, "Bearer "+testToken)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}

				var payload commentRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				if payload.Body != "dependencies updated" {
					t.Errorf("request body.Body = %q, want %q", payload.Body, "dependencies updated")
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":7,"body":"dependencies updated"}`))
			},
			wantComment: forge.Comment{ID: 7, Body: "dependencies updated"},
		},
		{
			name: "unauthorized",
			body: "dependencies updated",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			},
			wantErr:    true,
			wantErrSub: "401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := newTestClient(server)
			comment, err := client.CreateComment(context.Background(), tt.body)

			if tt.wantErr {
				if err == nil {
					t.Fatal("CreateComment() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("CreateComment() error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateComment() unexpected error: %v", err)
			}
			if comment != tt.wantComment {
				t.Errorf("CreateComment() = %+v, want %+v", comment, tt.wantComment)
			}
		})
	}
}

func TestUpdateComment(t *testing.T) {
	tests := []struct {
		name       string
		commentID  int
		body       string
		handler    http.HandlerFunc
		wantErr    bool
		wantErrSub string
	}{
		{
			name:      "success",
			commentID: 9,
			body:      "updated body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch {
					t.Errorf("method = %s, want PATCH", r.Method)
				}
				if want := "/repos/owner/repo/issues/comments/9"; r.URL.EscapedPath() != want {
					t.Errorf("path = %s, want %s", r.URL.EscapedPath(), want)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
					t.Errorf("Authorization = %q, want %q", got, "Bearer "+testToken)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}

				var payload commentRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				if payload.Body != "updated body" {
					t.Errorf("request body.Body = %q, want %q", payload.Body, "updated body")
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":9,"body":"updated body"}`))
			},
		},
		{
			name:      "unauthorized",
			commentID: 9,
			body:      "updated body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			},
			wantErr:    true,
			wantErrSub: "401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := newTestClient(server)
			err := client.UpdateComment(context.Background(), tt.commentID, tt.body)

			if tt.wantErr {
				if err == nil {
					t.Fatal("UpdateComment() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("UpdateComment() error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateComment() unexpected error: %v", err)
			}
		})
	}
}

func TestNewClient_DefaultHTTPClient(t *testing.T) {
	c := NewClient(testToken, testRepository, testPRNumber, nil)
	if c.httpClient != http.DefaultClient {
		t.Errorf("httpClient = %v, want http.DefaultClient", c.httpClient)
	}
	if c.baseURL != apiURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, apiURL)
	}
}

func TestDeleteComment(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		wantErrSub string
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("method = %s, want DELETE", r.Method)
				}
				// Like updates, GitHub addresses comment deletions by
				// repository rather than by pull request number.
				if want := "/repos/owner/repo/issues/comments/9"; r.URL.EscapedPath() != want {
					t.Errorf("path = %s, want %s", r.URL.EscapedPath(), want)
				}

				w.WriteHeader(http.StatusNoContent)
			},
		},
		{
			name: "forbidden",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			},
			wantErr:    true,
			wantErrSub: "403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			err := newTestClient(server).DeleteComment(context.Background(), 9)

			if tt.wantErr {
				if err == nil {
					t.Fatal("DeleteComment() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("DeleteComment() error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("DeleteComment() unexpected error: %v", err)
			}
		})
	}
}

func TestDescription(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		want       string
		wantErr    bool
		wantErrSub string
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				// The description lives on the pull request resource,
				// unlike comments, which GitHub models as issue comments.
				if want := "/repos/owner/repo/pulls/42"; r.URL.EscapedPath() != want {
					t.Errorf("path = %s, want %s", r.URL.EscapedPath(), want)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number":42,"body":"Closes #12."}`))
			},
			want: "Closes #12.",
		},
		{
			name: "null body decodes to the empty string",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number":42,"body":null}`))
			},
			want: "",
		},
		{
			name: "not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			},
			wantErr:    true,
			wantErrSub: "404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			got, err := newTestClient(server).Description(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("Description() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("Description() error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("Description() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Description() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpdateDescription(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		wantErrSub string
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch {
					t.Errorf("method = %s, want PATCH", r.Method)
				}
				if want := "/repos/owner/repo/pulls/42"; r.URL.EscapedPath() != want {
					t.Errorf("path = %s, want %s", r.URL.EscapedPath(), want)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}

				var payload pullRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				if payload.Body != "new description" {
					t.Errorf("request body = %q, want %q", payload.Body, "new description")
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"number":42,"body":"new description"}`))
			},
		},
		{
			name: "forbidden",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
			},
			wantErr:    true,
			wantErrSub: "403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			err := newTestClient(server).UpdateDescription(context.Background(), "new description")

			if tt.wantErr {
				if err == nil {
					t.Fatal("UpdateDescription() error = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("UpdateDescription() error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateDescription() unexpected error: %v", err)
			}
		})
	}
}
