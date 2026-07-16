package configuration_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/codeready-toolchain/toolchain-cicd/govulncheck-action/internal/configuration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfiguration(t *testing.T) {

	t.Run("empty filename", func(t *testing.T) {
		// when
		c, err := configuration.New("")
		// then
		require.NoError(t, err)
		assert.Empty(t, c.IgnoredVulnerabilities)
	})

	t.Run("empty file", func(t *testing.T) {
		// given
		tempFile, err := os.CreateTemp("", "ignored-vuln-*.yaml")
		require.NoError(t, err)
		// when
		c, err := configuration.New(tempFile.Name())
		// then
		require.NoError(t, err)
		assert.Empty(t, c.IgnoredVulnerabilities)
	})

	t.Run("empty ignored-vulnerabilities", func(t *testing.T) {
		// given
		tempFile, err := os.CreateTemp("", "ignored-vuln-*.yaml")
		fmt.Fprintln(tempFile, "ignored-vulnerabilities:")
		require.NoError(t, err)
		// when
		c, err := configuration.New(tempFile.Name())
		// then
		require.NoError(t, err)
		assert.Empty(t, c.IgnoredVulnerabilities)
	})

	t.Run("some ignored-vulnerabilities", func(t *testing.T) {
		// given
		tempFile, err := os.CreateTemp("", "ignored-vuln-*.yaml")
		require.NoError(t, err)

		content := `ignored-vulnerabilities:
    # Kubernetes kube-apiserver Vulnerable to Race Condition in k8s.io/kubernetes
    # More info: https://pkg.go.dev/vuln/GO-2025-3547
    # Module: k8s.io/kubernetes
    # Fixed in: N/A
    - id: GO-2025-3547
      silence-until: 2025-05-10
      info: https://pkg.go.dev/vuln/GO-2025-3547
    # Kubernetes GitRepo Volume Inadvertent Local Repository Access in k8s.io/kubernetes
    # More info: https://pkg.go.dev/vuln/GO-2025-3521
    # Module: k8s.io/kubernetes
    # Fixed in: N/A
    - id: GO-2025-3521
      silence-until: 2025-05-10
      info: https://pkg.go.dev/vuln/GO-2025-3521
    # Request smuggling due to acceptance of invalid chunked data in net/http
    # More info: https://pkg.go.dev/vuln/GO-2025-3563
    # Standard library
    # Fixed in: net/http/internal@go1.23.8
    - id: GO-2025-3563
      silence-until: 2025-05-10
      info: https://pkg.go.dev/vuln/GO-2025-3563`
		_, err = tempFile.WriteString(content)
		require.NoError(t, err)

		// when
		c, err := configuration.New(tempFile.Name())
		// then
		require.NoError(t, err)
		require.Len(t, c.IgnoredVulnerabilities, 3)
		assert.Equal(t, "GO-2025-3547", c.IgnoredVulnerabilities[0].ID)
		assert.Equal(t, "2025-05-10", c.IgnoredVulnerabilities[0].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "https://pkg.go.dev/vuln/GO-2025-3547", c.IgnoredVulnerabilities[0].Info)
		assert.Equal(t, "GO-2025-3521", c.IgnoredVulnerabilities[1].ID)
		assert.Equal(t, "2025-05-10", c.IgnoredVulnerabilities[1].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "https://pkg.go.dev/vuln/GO-2025-3521", c.IgnoredVulnerabilities[1].Info)
		assert.Equal(t, "GO-2025-3563", c.IgnoredVulnerabilities[2].ID)
		assert.Equal(t, "2025-05-10", c.IgnoredVulnerabilities[2].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "https://pkg.go.dev/vuln/GO-2025-3563", c.IgnoredVulnerabilities[2].Info)
	})

	t.Run("save and reload", func(t *testing.T) {
		// given
		tempFile, err := os.CreateTemp("", "ignored-vuln-*.yaml")
		require.NoError(t, err)
		silenceUntil := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
		cfg := configuration.Configuration{
			IgnoredVulnerabilities: []*configuration.Vulnerability{
				{
					ID:           "GO-2025-0001",
					SilenceUntil: silenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0001",
					Comment:      "Some vulnerability summary\nFound in: pkg@v1.0.0\nFixed in: pkg@v1.0.1",
				},
				{
					ID:           "GO-2025-0002",
					SilenceUntil: silenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0002",
				},
			},
		}
		// when
		err = configuration.Save(tempFile.Name(), cfg)
		// then
		require.NoError(t, err)
		// verify by reading back
		loaded, err := configuration.New(tempFile.Name())
		require.NoError(t, err)
		require.Len(t, loaded.IgnoredVulnerabilities, 2)
		assert.Equal(t, "GO-2025-0001", loaded.IgnoredVulnerabilities[0].ID)
		assert.Equal(t, "2025-06-15", loaded.IgnoredVulnerabilities[0].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "https://pkg.go.dev/vuln/GO-2025-0001", loaded.IgnoredVulnerabilities[0].Info)
		assert.Equal(t, "GO-2025-0002", loaded.IgnoredVulnerabilities[1].ID)
		assert.Equal(t, "2025-06-15", loaded.IgnoredVulnerabilities[1].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "https://pkg.go.dev/vuln/GO-2025-0002", loaded.IgnoredVulnerabilities[1].Info)
		// verify comments are in the raw file
		raw, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		assert.Contains(t, string(raw), "Some vulnerability summary")
		assert.Contains(t, string(raw), "Found in: pkg@v1.0.0")
	})

	t.Run("save empty config", func(t *testing.T) {
		// given
		tempFile, err := os.CreateTemp("", "ignored-vuln-*.yaml")
		require.NoError(t, err)
		cfg := configuration.Configuration{}
		// when
		err = configuration.Save(tempFile.Name(), cfg)
		// then
		require.NoError(t, err)
		loaded, err := configuration.New(tempFile.Name())
		require.NoError(t, err)
		assert.Empty(t, loaded.IgnoredVulnerabilities)
	})

	t.Run("invalid file", func(t *testing.T) {
		// given
		tempFile, err := os.CreateTemp("", "ignored-vuln-*.yaml")
		fmt.Fprintln(tempFile, "ignored-vulnerabilities:")
		fmt.Fprintln(tempFile, "GO-2025-3547")
		require.NoError(t, err)
		// when
		_, err = configuration.New(tempFile.Name())
		// then
		require.Error(t, err)
	})
}
