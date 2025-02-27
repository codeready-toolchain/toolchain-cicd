package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/codeready-toolchain/pairing/pkg/cmd/flags"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v44/github"
	"github.com/spf13/cobra"
)

type PairingServiceInterface interface {
	shouldPair(orgForPairing, repoForPairing, currentRemoteName, currentBranchName string) (bool, error)
}

type PairingService struct{}

func (s *PairingService) shouldPair(orgForPairing, repoForPairing, currentRemoteName, currentBranchName string) (bool, error) {
	return shouldPair(orgForPairing, repoForPairing, currentRemoteName, currentBranchName)
}

func NewPairCmd() *cobra.Command {
	var cloneDir, organization, repository string

	command := &cobra.Command{
		Use:   "pair --clone-dir=<clone-dir> --organization=<organization> --repository=<repository>",
		Short: "Pair PRs in CI between the given organization and repository into the given clone directory",
		Long:  `Automatically tries to pair a PR opened on a specific repository with a branch of the same name that potentially could exist in the given organization and repository.`,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Pair(cloneDir, organization, repository, &PairingService{})
		},
	}

	command.Flags().StringVarP(&cloneDir, "clone-dir", "c", "", "Directory to clone into")
	flags.MustMarkRequired(command, "clone-dir")

	command.Flags().StringVarP(&organization, "organization", "o", "", "Organization to pair")
	flags.MustMarkRequired(command, "organization")

	command.Flags().StringVarP(&repository, "repository", "r", "", "Repository to pair")
	flags.MustMarkRequired(command, "repository")

	return command
}

// listOpenPRs lists open pull requests for the given repository
func listOpenPRs(owner, repo string) ([]*github.PullRequest, error) {
	client := github.NewClient(nil) // no authentication needed

	// list the open pull requests
	opt := &github.PullRequestListOptions{
		State: "open",
	}

	prs, _, err := client.PullRequests.List(context.Background(), owner, repo, opt)
	if err != nil {
		return nil, fmt.Errorf("could not list pull requests: %w", err)
	}

	return prs, nil
}

// shouldPair determines whether the given remote and branch name should be paired
// based on existing open pull requests in the given organization and repository.
func shouldPair(orgForPairing, repoForPairing, currentRemoteName, currentBranchName string) (bool, error) {
	pullRequests, err := listOpenPRs(orgForPairing, repoForPairing)
	if err != nil {
		return false, err
	}

	for _, pull := range pullRequests {
		if pull.GetHead().GetRef() == currentBranchName && pull.GetUser().GetLogin() == currentRemoteName {
			return true, nil
		}
	}

	return false, nil
}

// getCurrentPRInfo gets the current info of the PR that triggered the pairing
// the pairing can be triggered by CI job or GH action
func getCurrentPRInfo() (*PullRequestMetadata, error) {
	pr := &PullRequestMetadata{}
	jobSpecEnvVarData := os.Getenv("JOB_SPEC")

	// running CI job
	if jobSpecEnvVarData != "" {
		log.Println("running in CI job")
		jobSpec := &JobSpec{}
		if err := json.Unmarshal([]byte(jobSpecEnvVarData), jobSpec); err != nil {
			return pr, fmt.Errorf("error when parsing openshift job spec data: %w", err)
		}
		if len(jobSpec.Refs.Pulls) == 1 {
			pull := jobSpec.Refs.Pulls[0]
			pr = &PullRequestMetadata{
				RemoteName: pull.Author,
				Number:     strconv.Itoa(pull.Number),
				BranchName: pull.HeadRef,
			}
		} else {
			fmt.Println("No pull request data found.")
		}
		// running GH action
	} else if os.Getenv("GITHUB_ACTIONS") != "" {
		log.Println("running in GH action")
		pr = &PullRequestMetadata{
			RemoteName: os.Getenv("AUTHOR"),
			BranchName: os.Getenv("GITHUB_HEAD_REF"),
		}
	}

	return pr, nil
}

func clone(cloneDir, org, repo, prRemoteName, prBranchName string, p PairingServiceInterface) error {
	branch := "master"

	cloneDirInfo, err := os.Stat(cloneDir)
	if !os.IsNotExist(err) && cloneDirInfo.IsDir() {
		log.Printf("folder %s already exists... removing", cloneDir)

		err := os.RemoveAll(cloneDir)
		if err != nil {
			return fmt.Errorf("error removing %s folder: %w", cloneDir, err)
		}
	}

	// if CI
	if prRemoteName != "" && prBranchName != "" {
		shouldPair, err := p.shouldPair(org, repo, prRemoteName, prBranchName)
		if err != nil {
			return err
		}
		// check if pairing is required
		if shouldPair {
			org = prRemoteName
			branch = prBranchName
		}
	}

	url := fmt.Sprintf("https://github.com/%s/%s", org, repo)
	refName := fmt.Sprintf("refs/heads/%s", branch)

	log.Printf("cloning '%s' with git ref '%s'", url, refName)

	_, err = git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.ReferenceName(refName),
		Progress:      os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}

func Pair(cloneDir, org, repo string, p PairingServiceInterface) error {
	prBranchName, prRemoteName := "", ""
	if os.Getenv("CI") == "true" {
		pr, err := getCurrentPRInfo()
		if err != nil {
			return err
		}
		prBranchName, prRemoteName = pr.BranchName, pr.RemoteName
	}

	return clone(cloneDir, org, repo, prRemoteName, prBranchName, p)
}
