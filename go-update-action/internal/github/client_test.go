package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ghClient := gh.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	ghClient.BaseURL = u
	ghClient.UploadURL = u

	return newTestClient(ghClient, "test-owner", "test-repo")
}

func TestGetDefaultBranch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/test-owner/test-repo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gh.Repository{
			DefaultBranch: gh.Ptr("main"),
		})
	})

	client := setupTestClient(t, mux)
	branch, err := client.GetDefaultBranch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestFindOpenPRByLabel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/test-owner/test-repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "open", r.URL.Query().Get("state"))
		json.NewEncoder(w).Encode([]*gh.PullRequest{
			{
				Number: gh.Ptr(1),
				Title:  gh.Ptr("chore: bump Go from 1.26.0 to 1.26.1"),
				Labels: []*gh.Label{{Name: gh.Ptr("go-update")}},
			},
			{
				Number: gh.Ptr(2),
				Title:  gh.Ptr("unrelated PR"),
				Labels: []*gh.Label{{Name: gh.Ptr("bug")}},
			},
			{
				Number: gh.Ptr(3),
				Title:  gh.Ptr("chore: bump Go from 1.25.0 to 1.26.0"),
				Labels: []*gh.Label{{Name: gh.Ptr("go-update")}, {Name: gh.Ptr("enhancement")}},
			},
		})
	})

	client := setupTestClient(t, mux)
	prs, err := client.FindOpenPRByLabel(context.Background(), "go-update")
	require.NoError(t, err)
	assert.Len(t, prs, 2)
	assert.Equal(t, 1, prs[0].GetNumber())
	assert.Equal(t, 3, prs[1].GetNumber())
}

func TestClosePR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /repos/test-owner/test-repo/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var pr gh.PullRequest
		json.Unmarshal(body, &pr)
		assert.Equal(t, "closed", pr.GetState())
		json.NewEncoder(w).Encode(gh.PullRequest{
			Number: gh.Ptr(42),
			State:  gh.Ptr("closed"),
		})
	})

	client := setupTestClient(t, mux)
	err := client.ClosePR(context.Background(), 42)
	require.NoError(t, err)
}

func TestCreateBranchFromDefault(t *testing.T) {
	t.Run("branch does not exist", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/test-owner/test-repo", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(gh.Repository{
				DefaultBranch: gh.Ptr("main"),
			})
		})
		mux.HandleFunc("GET /repos/test-owner/test-repo/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(gh.Reference{
				Ref:    gh.Ptr("refs/heads/main"),
				Object: &gh.GitObject{SHA: gh.Ptr("abc123")},
			})
		})
		mux.HandleFunc("GET /repos/test-owner/test-repo/git/ref/heads/go-update-1.26.1", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(gh.ErrorResponse{
				Message: "Not Found",
			})
		})
		var createdRef gh.Reference
		mux.HandleFunc("POST /repos/test-owner/test-repo/git/refs", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &createdRef)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(gh.Reference{
				Ref:    createdRef.Ref,
				Object: createdRef.Object,
			})
		})

		client := setupTestClient(t, mux)
		sha, err := client.CreateBranchFromDefault(context.Background(), "go-update-1.26.1")
		require.NoError(t, err)
		assert.Equal(t, "abc123", sha)
		assert.Equal(t, "refs/heads/go-update-1.26.1", createdRef.GetRef())
	})

	t.Run("branch already exists", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/test-owner/test-repo", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(gh.Repository{
				DefaultBranch: gh.Ptr("main"),
			})
		})
		mux.HandleFunc("GET /repos/test-owner/test-repo/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(gh.Reference{
				Ref:    gh.Ptr("refs/heads/main"),
				Object: &gh.GitObject{SHA: gh.Ptr("def456")},
			})
		})
		mux.HandleFunc("GET /repos/test-owner/test-repo/git/ref/heads/go-update-1.26.1", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(gh.Reference{
				Ref:    gh.Ptr("refs/heads/go-update-1.26.1"),
				Object: &gh.GitObject{SHA: gh.Ptr("old-sha")},
			})
		})
		var updatedRef gh.Reference
		mux.HandleFunc("PATCH /repos/test-owner/test-repo/git/refs/heads/go-update-1.26.1", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &updatedRef)
			json.NewEncoder(w).Encode(gh.Reference{
				Ref:    gh.Ptr("refs/heads/go-update-1.26.1"),
				Object: &gh.GitObject{SHA: gh.Ptr("def456")},
			})
		})

		client := setupTestClient(t, mux)
		sha, err := client.CreateBranchFromDefault(context.Background(), "go-update-1.26.1")
		require.NoError(t, err)
		assert.Equal(t, "def456", sha)
	})
}

func TestCreateCommit(t *testing.T) {
	mux := http.NewServeMux()

	var createdBlobs []string
	mux.HandleFunc("POST /repos/test-owner/test-repo/git/blobs", func(w http.ResponseWriter, r *http.Request) {
		var blob gh.Blob
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &blob)
		sha := "blob-" + blob.GetContent()[:5]
		createdBlobs = append(createdBlobs, sha)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gh.Blob{SHA: gh.Ptr(sha)})
	})

	mux.HandleFunc("GET /repos/test-owner/test-repo/git/commits/base-sha", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gh.Commit{
			SHA:  gh.Ptr("base-sha"),
			Tree: &gh.Tree{SHA: gh.Ptr("base-tree-sha")},
		})
	})

	var treeEntries []*gh.TreeEntry
	mux.HandleFunc("POST /repos/test-owner/test-repo/git/trees", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			BaseTree string          `json:"base_tree"`
			Entries  []*gh.TreeEntry `json:"tree"`
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)
		treeEntries = req.Entries
		assert.Equal(t, "base-tree-sha", req.BaseTree)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gh.Tree{SHA: gh.Ptr("new-tree-sha")})
	})

	mux.HandleFunc("POST /repos/test-owner/test-repo/git/commits", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gh.Commit{SHA: gh.Ptr("new-commit-sha")})
	})

	mux.HandleFunc("PATCH /repos/test-owner/test-repo/git/refs/heads/go-update-1.26.1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gh.Reference{
			Ref:    gh.Ptr("refs/heads/go-update-1.26.1"),
			Object: &gh.GitObject{SHA: gh.Ptr("new-commit-sha")},
		})
	})

	client := setupTestClient(t, mux)
	sha, err := client.CreateCommit(context.Background(), "go-update-1.26.1", "base-sha", "chore: bump Go", []FileChange{
		{Path: "go.mod", Content: "module example\n\ngo 1.26.1\n"},
		{Path: "Dockerfile", Content: "FROM golang:1.26.1\n"},
	})

	require.NoError(t, err)
	assert.Equal(t, "new-commit-sha", sha)
	assert.Len(t, createdBlobs, 2)
	assert.Len(t, treeEntries, 2)
	assert.Equal(t, "go.mod", treeEntries[0].GetPath())
	assert.Equal(t, "Dockerfile", treeEntries[1].GetPath())
}

func TestCreatePR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/test-owner/test-repo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(gh.Repository{
			DefaultBranch: gh.Ptr("main"),
		})
	})

	var createdPR gh.NewPullRequest
	mux.HandleFunc("POST /repos/test-owner/test-repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &createdPR)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gh.PullRequest{
			Number:  gh.Ptr(99),
			HTMLURL: gh.Ptr("https://github.com/test-owner/test-repo/pull/99"),
		})
	})

	var addedLabels []string
	mux.HandleFunc("POST /repos/test-owner/test-repo/issues/99/labels", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &addedLabels)
		json.NewEncoder(w).Encode([]*gh.Label{
			{Name: gh.Ptr("go-update")},
		})
	})

	client := setupTestClient(t, mux)
	prURL, err := client.CreatePR(context.Background(),
		"chore: bump Go from 1.26.0 to 1.26.1",
		"Update Go version",
		"go-update-1.26.1",
		[]string{"go-update"},
	)

	require.NoError(t, err)
	assert.Equal(t, "https://github.com/test-owner/test-repo/pull/99", prURL)
	assert.Equal(t, "chore: bump Go from 1.26.0 to 1.26.1", createdPR.GetTitle())
	assert.Equal(t, "go-update-1.26.1", createdPR.GetHead())
	assert.Equal(t, "main", createdPR.GetBase())
	assert.Contains(t, addedLabels, "go-update")
}

func TestDeleteBranch(t *testing.T) {
	mux := http.NewServeMux()
	deleted := false
	mux.HandleFunc("DELETE /repos/test-owner/test-repo/git/refs/heads/go-update-1.26.1", func(w http.ResponseWriter, r *http.Request) {
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	})

	client := setupTestClient(t, mux)
	err := client.DeleteBranch(context.Background(), "go-update-1.26.1")
	require.NoError(t, err)
	assert.True(t, deleted)
}
