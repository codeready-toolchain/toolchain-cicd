package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/codeready-toolchain/pairing/pkg/cmd/flags"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

type ApplyPairFunc func(repo *git.Repository, forkRepoURL, remoteBranch string) error

func NewPairCmd() *cobra.Command {
	var cloneDir, organization, repository string

	command := &cobra.Command{
		Use:   "pair --clone-dir=<clone-dir> --organization=<organization> --repository=<repository>",
		Short: "Pair PRs in CI between the given organization and repository into the given clone directory",
		Long:  `Automatically tries to pair a PR opened on a specific repository with a branch of the same name that potentially could exist in the given organization and repository.`,
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Pair(cloneDir, organization, repository, applyPair)
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

// shouldPair checks if a branch with the same ref exists in the user's fork of the given repository
func shouldPair(forkRepoURL, branchForParing string) (bool, error) {
	url := fmt.Sprintf("%s/info/refs?service=git-upload-pack", forkRepoURL)

	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if resp.StatusCode != 200 {
		return false, fmt.Errorf("failed to get repo: %s", string(body))
	}

	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == fmt.Sprintf("refs/heads/%s", branchForParing) {
			return true, nil
		}
	}
	// branch not found
	return false, nil
}

// getCurrentPRInfo gets the current info of the PR that triggered the pairing
// the pairing can be triggered by OpenShift-CI job or GH action
func getCurrentPRInfo() (string, string, error) {
	// JOB_SPEC contains all the info about the running OpenShift-CI job,
	// including the refs (org, repo, pulls, etc.)
	jobSpecEnvVarData := os.Getenv("JOB_SPEC")

	// running OpenShift-CI job
	if jobSpecEnvVarData != "" {
		log.Println("running in OpenShift-CI job")
		jobSpec := &JobSpec{}
		if err := json.Unmarshal([]byte(jobSpecEnvVarData), jobSpec); err != nil {
			return "", "", fmt.Errorf("error when parsing openshift job spec data: %w", err)
		}
		if len(jobSpec.Refs.Pulls) == 1 {
			pull := jobSpec.Refs.Pulls[0]
			return pull.Author, pull.HeadRef, nil
		} else {
			return "", "", fmt.Errorf("no pull request data found or more than one pull found")
		}
		// running GH action
	} else if os.Getenv("GITHUB_ACTIONS") != "" {
		log.Println("running in GH action")
		return os.Getenv("AUTHOR"), os.Getenv("GITHUB_HEAD_REF"), nil
	}

	return "", "", fmt.Errorf("not running in OpenShift-CI job either GH action")
}

func clone(cloneDir, url string) (*git.Repository, error) {
	cloneDirInfo, err := os.Stat(cloneDir)

	if err != nil && !os.IsNotExist(err) {
		if cloneDirInfo.IsDir() {
			log.Printf("folder %s already exists... removing", cloneDir)

			err := os.RemoveAll(cloneDir)
			if err != nil {
				return nil, fmt.Errorf("error removing %s folder: %w", cloneDir, err)
			}
		} else {
			return nil, fmt.Errorf("cloneDir %s provided is not a directory", cloneDir)
		}
	}

	repo, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: "refs/heads/master",
		// Depth:         1,
		Progress: os.Stdout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	return repo, nil
}

func applyPair(repo *git.Repository, forkRepoURL, remoteBranch string) error {
	log.Printf("branch ref of the user's fork (%s) to be used for pairing: %s\n", forkRepoURL, remoteBranch)

	// add the user's fork as remote
	// git remote add external forkRepoURL
	remote, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "external",
		URLs: []string{forkRepoURL},
	})
	if err != nil {
		return fmt.Errorf("failed to add remote: %w", err)
	}

	// fetch the remote branch
	// git fetch external remoteBranch
	remoteRefName := plumbing.NewRemoteReferenceName(remote.Config().Name, remoteBranch)
	err = repo.Fetch(&git.FetchOptions{
		RemoteName: remote.Config().Name,
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/%s:%s", remoteBranch, remoteRefName)),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	reference, err := repo.Reference(remoteRefName, true)
	if err != nil {
		return fmt.Errorf("fetched branch does not exist: %w", err)
	}

	// merge the remote branch with master
	// git merge remoteBranch
	err = repo.Merge(*reference, git.MergeOptions{Strategy: git.FastForwardMerge})
	if err != nil {
		return fmt.Errorf("failed to merge: %w", err)
	}

	return err
}

func Pair(cloneDir, organization, repository string, applyPair ApplyPairFunc) error {
	parentRepoURL := fmt.Sprintf("https://github.com/%s/%s.git", organization, repository)
	log.Printf("cloning parent repo %s", parentRepoURL)
	repo, err := clone(cloneDir, parentRepoURL)
	if err != nil {
		return err
	}

	// running in CI
	if os.Getenv("CI") == "true" {
		authorName, remoteBranch, err := getCurrentPRInfo()
		if err != nil {
			return err
		}

		forkRepoURL := fmt.Sprintf("https://github.com/%s/%s.git", authorName, repository)

		shouldPair, err := shouldPair(forkRepoURL, remoteBranch)
		if err != nil {
			return err
		}

		if shouldPair {
			return applyPair(repo, forkRepoURL, remoteBranch)
		}

		log.Println("running in CI but no pairing needed")
		return nil
	}

	// not running in CI
	log.Println("not running in CI, so pairing is not needed")
	return nil
}
