package github

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v72/github"
)

type FileChange struct {
	Path    string
	Content string
}

type PRResult struct {
	URL         string
	ActionTaken string // "created", "skipped", "replaced"
}

type Client struct {
	client *gh.Client
	owner  string
	repo   string
}

func NewClient(_ context.Context, token, owner, repo string) *Client {
	client := gh.NewClient(nil).WithAuthToken(token)
	return &Client{client: client, owner: owner, repo: repo}
}

func newTestClient(ghClient *gh.Client, owner, repo string) *Client {
	return &Client{client: ghClient, owner: owner, repo: repo}
}

func (c *Client) GetDefaultBranch(ctx context.Context) (string, error) {
	repo, _, err := c.client.Repositories.Get(ctx, c.owner, c.repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository: %w", err)
	}
	return repo.GetDefaultBranch(), nil
}

func (c *Client) FindOpenPRByLabel(ctx context.Context, label string) ([]*gh.PullRequest, error) {
	var allPRs []*gh.PullRequest
	opts := &gh.PullRequestListOptions{
		State: "open",
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}
	for {
		prs, resp, err := c.client.PullRequests.List(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests: %w", err)
		}
		for _, pr := range prs {
			for _, l := range pr.Labels {
				if l.GetName() == label {
					allPRs = append(allPRs, pr)
					break
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allPRs, nil
}

func (c *Client) ClosePR(ctx context.Context, number int) error {
	state := "closed"
	_, _, err := c.client.PullRequests.Edit(ctx, c.owner, c.repo, number, &gh.PullRequest{
		State: &state,
	})
	if err != nil {
		return fmt.Errorf("failed to close PR #%d: %w", number, err)
	}
	return nil
}

func (c *Client) CreateBranchFromDefault(ctx context.Context, branchName string) (string, error) {
	defaultBranch, err := c.GetDefaultBranch(ctx)
	if err != nil {
		return "", err
	}

	defaultRef, _, err := c.client.Git.GetRef(ctx, c.owner, c.repo, "refs/heads/"+defaultBranch)
	if err != nil {
		return "", fmt.Errorf("failed to get default branch ref: %w", err)
	}
	baseSHA := defaultRef.GetObject().GetSHA()

	ref := &gh.Reference{
		Ref: gh.Ptr("refs/heads/" + branchName),
		Object: &gh.GitObject{
			SHA: &baseSHA,
		},
	}

	_, resp, err := c.client.Git.GetRef(ctx, c.owner, c.repo, "refs/heads/"+branchName)
	if err == nil {
		// branch exists, update it
		_, _, err = c.client.Git.UpdateRef(ctx, c.owner, c.repo, ref, true)
		if err != nil {
			return "", fmt.Errorf("failed to update branch ref: %w", err)
		}
		return baseSHA, nil
	}

	if resp.StatusCode != 404 {
		return "", fmt.Errorf("failed to check branch existence: %w", err)
	}

	_, _, err = c.client.Git.CreateRef(ctx, c.owner, c.repo, ref)
	if err != nil {
		return "", fmt.Errorf("failed to create branch ref: %w", err)
	}
	return baseSHA, nil
}

func (c *Client) CreateCommit(ctx context.Context, branch, baseSHA, message string, changes []FileChange) (string, error) {
	entries := make([]*gh.TreeEntry, 0, len(changes))
	for _, change := range changes {
		blob, _, err := c.client.Git.CreateBlob(ctx, c.owner, c.repo, &gh.Blob{
			Content:  gh.Ptr(change.Content),
			Encoding: gh.Ptr("utf-8"),
		})
		if err != nil {
			return "", fmt.Errorf("failed to create blob for %s: %w", change.Path, err)
		}
		entries = append(entries, &gh.TreeEntry{
			Path: gh.Ptr(change.Path),
			Mode: gh.Ptr("100644"),
			Type: gh.Ptr("blob"),
			SHA:  blob.SHA,
		})
	}

	baseCommit, _, err := c.client.Git.GetCommit(ctx, c.owner, c.repo, baseSHA)
	if err != nil {
		return "", fmt.Errorf("failed to get base commit: %w", err)
	}

	tree, _, err := c.client.Git.CreateTree(ctx, c.owner, c.repo, baseCommit.GetTree().GetSHA(), entries)
	if err != nil {
		return "", fmt.Errorf("failed to create tree: %w", err)
	}

	commit, _, err := c.client.Git.CreateCommit(ctx, c.owner, c.repo, &gh.Commit{
		Message: gh.Ptr(message),
		Tree:    tree,
		Parents: []*gh.Commit{{SHA: &baseSHA}},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	commitSHA := commit.GetSHA()
	ref := &gh.Reference{
		Ref: gh.Ptr("refs/heads/" + branch),
		Object: &gh.GitObject{
			SHA: &commitSHA,
		},
	}
	_, _, err = c.client.Git.UpdateRef(ctx, c.owner, c.repo, ref, false)
	if err != nil {
		return "", fmt.Errorf("failed to update branch ref: %w", err)
	}

	return commitSHA, nil
}

func (c *Client) CreatePR(ctx context.Context, title, body, branch string, labels []string) (string, error) {
	defaultBranch, err := c.GetDefaultBranch(ctx)
	if err != nil {
		return "", err
	}

	pr, _, err := c.client.PullRequests.Create(ctx, c.owner, c.repo, &gh.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &branch,
		Base:  &defaultBranch,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create pull request: %w", err)
	}

	if len(labels) > 0 {
		_, _, err = c.client.Issues.AddLabelsToIssue(ctx, c.owner, c.repo, pr.GetNumber(), labels)
		if err != nil {
			return "", fmt.Errorf("failed to add labels to PR: %w", err)
		}
	}

	return pr.GetHTMLURL(), nil
}

func (c *Client) DeleteBranch(ctx context.Context, branch string) error {
	_, err := c.client.Git.DeleteRef(ctx, c.owner, c.repo, "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branch, err)
	}
	return nil
}
