package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock the shouldPair function
type MockPairingService struct {
	mock.Mock
}

func (m *MockPairingService) shouldPair(forkRepoURL, branchForParing string) (bool, error) {
	return true, nil
}

func TestPair(t *testing.T) {
	t.Run("error during clone: repository not found", func(t *testing.T) {
		expectedError := fmt.Errorf("failed to clone repository: authentication required: Repository not found.")
		pair(t, "/tmp/repository-not-found", "codeready-toolchain", "host-operato", expectedError, &PairingService{})
	})

	t.Run("not running in ci", func(t *testing.T) {
		pair(t, "/tmp/not-running-in-ci", "codeready-toolchain", "host-operator", nil, &PairingService{})
	})

	t.Run("running in ci - gh action", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("AUTHOR", "rsoaresd")
		t.Setenv("GITHUB_HEAD_REF", "clean_only_when_test_passed")

		pair(t, "/tmp/running-in-gh-action", "kubesaw", "ksctl", nil, &PairingService{})
	})

	t.Run("running in ci - prow job", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("JOB_SPEC", `{"type":"presubmit","job":"pull-ci-codeready-toolchain-toolchain-e2e-master-e2e","buildid":"1889023022812106752","prowjobid":"9ccb229f-aebf-45d6-90e2-2388663e8b9a","refs":{"org":"codeready-toolchain","repo":"toolchain-e2e","repo_link":"https://github.com/codeready-toolchain/toolchain-e2e","base_ref":"master","base_sha":"47ac08434063871caf78c8f3d6dbab6df61ecb63","base_link":"https://github.com/codeready-toolchain/toolchain-e2e/commit/47ac08434063871caf78c8f3d6dbab6df61ecb63","pulls":[{"number":1113,"author":"rsoaresd","sha":"67ece9d9716bcc8556f91c0c909ecea4b7c17bff","head_ref":"test-pairing","link":"https://github.com/codeready-toolchain/toolchain-e2e/pull/1113","commit_link":"https://github.com/codeready-toolchain/toolchain-e2e/pull/1113/commits/67ece9d9716bcc8556f91c0c909ecea4b7c17bff","author_link":"https://github.com/rsoaresd"}]},"decoration_config":{"timeout":"2h0m0s","grace_period":"15s","utility_images":{"clonerefs":"us-docker.pkg.dev/k8s-infra-prow/images/clonerefs:v20250205-e871edfd1","initupload":"us-docker.pkg.dev/k8s-infra-prow/images/initupload:v20250205-e871edfd1","entrypoint":"us-docker.pkg.dev/k8s-infra-prow/images/entrypoint:v20250205-e871edfd1","sidecar":"us-docker.pkg.dev/k8s-infra-prow/images/sidecar:v20250205-e871edfd1"},"resources":{"clonerefs":{"limits":{"memory":"3Gi"},"requests":{"cpu":"100m","memory":"500Mi"}},"initupload":{"limits":{"memory":"200Mi"},"requests":{"cpu":"100m","memory":"50Mi"}},"place_entrypoint":{"limits":{"memory":"100Mi"},"requests":{"cpu":"100m","memory":"25Mi"}},"sidecar":{"limits":{"memory":"2Gi"},"requests":{"cpu":"100m","memory":"250Mi"}}},"gcs_configuration":{"bucket":"test-platform-results","path_strategy":"single","default_org":"openshift","default_repo":"origin","mediaTypes":{"log":"text/plain"},"compress_file_types":["txt","log","json","tar","html","yaml"]},"gcs_credentials_secret":"gce-sa-credentials-gcs-publisher","skip_cloning":true,"censor_secrets":true}}`)

		pair(t, "/tmp/running-in-ci-prow-job", "codeready-toolchain", "host-operator", nil, &PairingService{})

	})

	t.Run("error parsing openshift job spec data", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("JOB_SPEC", `"type"`)
		expectedError := fmt.Errorf("error when parsing openshift job spec data: json: cannot unmarshal string into Go value of type cmd.JobSpec")

		pair(t, "/tmp/running-in-ci-prow-job", "codeready-toolchain", "host-operator", expectedError, &PairingService{})
	})

	t.Run("should pair", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("AUTHOR", "rsoaresd")
		t.Setenv("GITHUB_HEAD_REF", "master")

		pairingServiceMock := new(MockPairingService)
		pair(t, "/tmp/should-pair", "codeready-toolchain", "host-operator", nil, pairingServiceMock)

	})
}

func pair(t *testing.T, cloneDir, org, repo string, expectedError error, p PairingServiceInterface) {
	err := Pair(cloneDir, org, repo, p)

	defer func() {
		if err := os.RemoveAll(cloneDir); err != nil {
			t.Fatalf("failed to remove test directory: %v", err)
		}
	}()

	if expectedError == nil {
		assert.NoError(t, err)
	} else {
		assert.EqualError(t, err, expectedError.Error())
	}
}
