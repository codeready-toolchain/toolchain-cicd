package cmd

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
)

func TestPair(t *testing.T) {
	t.Run("error during clone: repository not found", func(t *testing.T) {
		expectedError := fmt.Errorf("failed to clone repository: authentication required")
		temp := mkdirTemp(t, "repository-not-found-")
		pair(t, temp, "codeready-toolchain", "host-operato", expectedError, applyPair)
	})

	t.Run("error during clone: cloneDir provided is not a directory", func(t *testing.T) {
		file, err := os.CreateTemp("", "file-")
		require.NoError(t, err)
		defer file.Close()
		defer os.Remove(file.Name())

		expectedError := fmt.Errorf("failed to clone repository: path is not a directory: %s", file.Name())
		pair(t, file.Name(), "codeready-toolchain", "host-operator", expectedError, applyPair)
	})

	t.Run("not running in ci, cloneDir already exists (should clean it and return no error)", func(t *testing.T) {
		// folder already exists
		cloneDir := "/tmp/host-operator"
		require.NoError(t, os.Mkdir(cloneDir, os.ModePerm))

		pair(t, cloneDir, "codeready-toolchain", "host-operator", nil, applyPair)
	})

	t.Run("not running in ci", func(t *testing.T) {
		temp := mkdirTemp(t, "not-running-in-ci-")
		pair(t, temp, "codeready-toolchain", "host-operator", nil, applyPair)
	})

	t.Run("running in ci - gh action - no pairing", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("AUTHOR", "cosmic")
		t.Setenv("GITHUB_HEAD_REF", "branch-test")

		// pair not needed since it did not found 'branch-test' branch
		SetupGockWithCleanup(t, "/cosmic/ksctl.git/info/refs", "00484f0b3f2ae6b774416cc91e779cca4a8bb71af054 refs/heads/branch", http.StatusOK, "service", "git-upload-pack")

		temp := mkdirTemp(t, "running-in-ci-gh-action-")
		pair(t, temp, "kubesaw", "ksctl", nil, applyPair)
	})

	t.Run("running in ci - prow job - no pairing", func(t *testing.T) {
		t.Setenv("CI", "true")

		prowJob, err := os.ReadFile("testdata/prow_job.json")
		require.NoError(t, err)
		t.Setenv("JOB_SPEC", string(prowJob))

		// pair not needed since it did not found 'branch-test' branch
		SetupGockWithCleanup(t, "/cosmic/host-operator.git/info/refs", "00484f0b3f2ae6b774416cc91e779cca4a8bb71af054 refs/heads/branch", http.StatusOK, "service", "git-upload-pack")

		temp := mkdirTemp(t, "running-in-ci-prow-job-")
		pair(t, temp, "codeready-toolchain", "host-operator", nil, applyPair)

	})

	t.Run("running in ci - prow job - error parsing openshift job spec data", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("JOB_SPEC", `"type"`)
		expectedError := fmt.Errorf("error when parsing openshift job spec data: json: cannot unmarshal string into Go value of type cmd.JobSpec")

		temp := mkdirTemp(t, "running-in-ci-prow-job-")
		pair(t, temp, "codeready-toolchain", "host-operator", expectedError, applyPair)
	})

	t.Run("running in ci - should pair", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("AUTHOR", "cosmic")
		t.Setenv("GITHUB_HEAD_REF", "master")

		// pair needed since it found 'master' branch
		SetupGockWithCleanup(t, "/cosmic/host-operator.git/info/refs", "00484f0b3f2ae6b774416cc91e779cca4a8bb71af054 refs/heads/master", http.StatusOK, "service", "git-upload-pack")

		fakeApplyPair := func(repo *git.Repository, forkRepoURL, remoteBranch string) error {
			return nil
		}

		temp := mkdirTemp(t, "should-pair-")
		pair(t, temp, "codeready-toolchain", "host-operator", nil, fakeApplyPair)
	})

	t.Run("running in ci - failed to pair", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("AUTHOR", "rsoaesd")
		t.Setenv("GITHUB_HEAD_REF", "clean_only_when_test_passed")

		expectedError := fmt.Errorf("failed to get repo: Repository not found.")
		temp := mkdirTemp(t, "running-in-gh-action-")
		pair(t, temp, "kubesaw", "ksctl", expectedError, applyPair)
	})

	t.Run("running in ci - but not running in OpenShift-CI job either GH action", func(t *testing.T) {
		t.Setenv("CI", "true")

		expectedError := fmt.Errorf("not running in OpenShift-CI job either GH action")
		temp := mkdirTemp(t, "running-in-gh-action-")
		pair(t, temp, "kubesaw", "ksctl", expectedError, applyPair)
	})
}

func pair(t *testing.T, cloneDir, org, repo string, expectedError error, applyPair ApplyPairFunc) {
	err := Pair(cloneDir, org, repo, applyPair)

	defer func() {
		if err := os.RemoveAll(cloneDir); err != nil {
			t.Fatalf("failed to remove test directory: %v", err)
		}
	}()

	if expectedError == nil {
		require.NoError(t, err)
	} else {
		require.EqualError(t, err, expectedError.Error())
	}
}

func SetupGockWithCleanup(t *testing.T, path string, body string, statusCode int, matchKey, matchValue string) {
	gock.New("https://github.com").
		Get(path).
		MatchParam(matchKey, matchValue).
		Persist().
		Reply(statusCode).
		BodyString(body)
	t.Cleanup(gock.OffAll)
}

func mkdirTemp(t *testing.T, dirName string) string {
	temp, err := os.MkdirTemp("", dirName)
	require.NoError(t, err)
	return temp
}
