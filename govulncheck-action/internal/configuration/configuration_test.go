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

	t.Run("load preserves comments", func(t *testing.T) {
		// given
		tempFile, err := os.CreateTemp("", "ignored-vuln-*.yaml")
		require.NoError(t, err)

		content := `ignored-vulnerabilities:
  # Kubernetes kube-apiserver Vulnerable to Race Condition
  # Found in: k8s.io/kubernetes@v1.31.0
  # Fixed in: N/A
  - id: GO-2025-3547
    silence-until: 2025-05-10
    info: https://pkg.go.dev/vuln/GO-2025-3547
  # Request smuggling due to acceptance of invalid chunked data
  # Found in: net/http/internal@go1.22.12
  # Fixed in: net/http/internal@go1.23.8
  - id: GO-2025-3563
    silence-until: 2025-05-10
    info: https://pkg.go.dev/vuln/GO-2025-3563
  - id: GO-2025-0099
    silence-until: 2025-05-10
    info: https://pkg.go.dev/vuln/GO-2025-0099`
		_, err = tempFile.WriteString(content)
		require.NoError(t, err)

		// when
		c, err := configuration.New(tempFile.Name())

		// then
		require.NoError(t, err)
		require.Len(t, c.IgnoredVulnerabilities, 3)
		// first entry: comment preserved
		assert.Equal(t, "GO-2025-3547", c.IgnoredVulnerabilities[0].ID)
		assert.Equal(t, "Kubernetes kube-apiserver Vulnerable to Race Condition\nFound in: k8s.io/kubernetes@v1.31.0\nFixed in: N/A", c.IgnoredVulnerabilities[0].Comment)
		// second entry: comment preserved
		assert.Equal(t, "GO-2025-3563", c.IgnoredVulnerabilities[1].ID)
		assert.Equal(t, "Request smuggling due to acceptance of invalid chunked data\nFound in: net/http/internal@go1.22.12\nFixed in: net/http/internal@go1.23.8", c.IgnoredVulnerabilities[1].Comment)
		// third entry: no comment
		assert.Equal(t, "GO-2025-0099", c.IgnoredVulnerabilities[2].ID)
		assert.Empty(t, c.IgnoredVulnerabilities[2].Comment)
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

	t.Run("save with 2-space indentation", func(t *testing.T) {
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
					Comment:      "Some vulnerability\nFound in: pkg@v1.0.0",
				},
			},
		}
		// when
		err = configuration.Save(tempFile.Name(), cfg)
		// then
		require.NoError(t, err)
		raw, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		expected := `ignored-vulnerabilities:
  # Some vulnerability
  # Found in: pkg@v1.0.0
  - id: GO-2025-0001
    silence-until: 2025-06-15
    info: https://pkg.go.dev/vuln/GO-2025-0001
`
		assert.Equal(t, expected, string(raw))
	})

	t.Run("save preserves existing entries and comments when adding new entry", func(t *testing.T) {
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
					Comment:      "Existing vulnerability one\nModule: pkg1@v1.0.0\nFixed in: N/A",
				},
				{
					ID:           "GO-2025-0002",
					SilenceUntil: silenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0002",
					Comment:      "Existing vulnerability two\nModule: pkg2@v2.0.0\nFixed in: pkg2@v2.0.1",
				},
			},
		}
		// save initial config
		err = configuration.Save(tempFile.Name(), cfg)
		require.NoError(t, err)

		// add a new entry
		newSilenceUntil := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		cfg.IgnoredVulnerabilities = append(cfg.IgnoredVulnerabilities, &configuration.Vulnerability{
			ID:           "GO-2025-0003",
			SilenceUntil: newSilenceUntil,
			Info:         "https://pkg.go.dev/vuln/GO-2025-0003",
			Comment:      "New vulnerability three\nModule: pkg3@v3.0.0\nFixed in: pkg3@v3.0.1",
		})

		// when
		err = configuration.Save(tempFile.Name(), cfg)

		// then
		require.NoError(t, err)
		// verify all entries are reloaded correctly
		loaded, err := configuration.New(tempFile.Name())
		require.NoError(t, err)
		require.Len(t, loaded.IgnoredVulnerabilities, 3)
		assert.Equal(t, "GO-2025-0001", loaded.IgnoredVulnerabilities[0].ID)
		assert.Equal(t, "2025-06-15", loaded.IgnoredVulnerabilities[0].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "https://pkg.go.dev/vuln/GO-2025-0001", loaded.IgnoredVulnerabilities[0].Info)
		assert.Equal(t, "GO-2025-0002", loaded.IgnoredVulnerabilities[1].ID)
		assert.Equal(t, "2025-06-15", loaded.IgnoredVulnerabilities[1].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "https://pkg.go.dev/vuln/GO-2025-0002", loaded.IgnoredVulnerabilities[1].Info)
		assert.Equal(t, "GO-2025-0003", loaded.IgnoredVulnerabilities[2].ID)
		assert.Equal(t, "2025-09-01", loaded.IgnoredVulnerabilities[2].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "https://pkg.go.dev/vuln/GO-2025-0003", loaded.IgnoredVulnerabilities[2].Info)
		// verify all comments are preserved in the raw file
		raw, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		assert.Contains(t, string(raw), "# Existing vulnerability one")
		assert.Contains(t, string(raw), "# Module: pkg1@v1.0.0")
		assert.Contains(t, string(raw), "# Fixed in: N/A")
		assert.Contains(t, string(raw), "# Existing vulnerability two")
		assert.Contains(t, string(raw), "# Module: pkg2@v2.0.0")
		assert.Contains(t, string(raw), "# Fixed in: pkg2@v2.0.1")
		assert.Contains(t, string(raw), "# New vulnerability three")
		assert.Contains(t, string(raw), "# Module: pkg3@v3.0.0")
		assert.Contains(t, string(raw), "# Fixed in: pkg3@v3.0.1")
	})

	t.Run("save preserves comments after config update with new and outdated vulns", func(t *testing.T) {
		// given an initial config with 3 entries
		tempFile, err := os.CreateTemp("", "ignored-vuln-*.yaml")
		require.NoError(t, err)
		silenceUntil := time.Date(2025, 5, 10, 0, 0, 0, 0, time.UTC)
		initial := configuration.Configuration{
			IgnoredVulnerabilities: []*configuration.Vulnerability{
				{
					ID:           "GO-2025-0001",
					SilenceUntil: silenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0001",
					Comment:      "Vuln one summary\nFound in: pkg1@v1.0.0\nFixed in: N/A",
				},
				{
					ID:           "GO-2025-0002",
					SilenceUntil: silenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0002",
					Comment:      "Vuln two summary\nFound in: pkg2@v2.0.0\nFixed in: pkg2@v2.1.0",
				},
				{
					ID:           "GO-2025-0003",
					SilenceUntil: silenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0003",
					Comment:      "Vuln three summary\nFound in: pkg3@v3.0.0\nFixed in: N/A",
				},
			},
		}
		err = configuration.Save(tempFile.Name(), initial)
		require.NoError(t, err)

		// simulate UpdateConfig result: keep 0001, remove 0002 (outdated), keep 0003 with updated comment, add 0004 (new)
		newSilenceUntil := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		updated := configuration.Configuration{
			IgnoredVulnerabilities: []*configuration.Vulnerability{
				{
					ID:           "GO-2025-0001",
					SilenceUntil: silenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0001",
					Comment:      "Vuln one summary\nFound in: pkg1@v1.0.0\nFixed in: N/A",
				},
				{
					ID:           "GO-2025-0003",
					SilenceUntil: newSilenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0003",
					Comment:      "Vuln three updated summary\nFound in: pkg3@v3.0.0\nFixed in: pkg3@v3.1.0",
				},
				{
					ID:           "GO-2025-0004",
					SilenceUntil: newSilenceUntil,
					Info:         "https://pkg.go.dev/vuln/GO-2025-0004",
					Comment:      "Brand new vuln four\nFound in: pkg4@v4.0.0\nFixed in: pkg4@v4.0.1",
				},
			},
		}

		// when
		err = configuration.Save(tempFile.Name(), updated)

		// then
		require.NoError(t, err)
		// verify entries reload correctly
		loaded, err := configuration.New(tempFile.Name())
		require.NoError(t, err)
		require.Len(t, loaded.IgnoredVulnerabilities, 3)
		assert.Equal(t, "GO-2025-0001", loaded.IgnoredVulnerabilities[0].ID)
		assert.Equal(t, "2025-05-10", loaded.IgnoredVulnerabilities[0].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "GO-2025-0003", loaded.IgnoredVulnerabilities[1].ID)
		assert.Equal(t, "2025-09-01", loaded.IgnoredVulnerabilities[1].SilenceUntil.Format("2006-01-02"))
		assert.Equal(t, "GO-2025-0004", loaded.IgnoredVulnerabilities[2].ID)
		assert.Equal(t, "2025-09-01", loaded.IgnoredVulnerabilities[2].SilenceUntil.Format("2006-01-02"))
		// verify comments in raw file
		raw, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		rawStr := string(raw)
		// kept entry: original comment preserved
		assert.Contains(t, rawStr, "# Vuln one summary")
		assert.Contains(t, rawStr, "# Found in: pkg1@v1.0.0")
		assert.Contains(t, rawStr, "# Fixed in: N/A")
		// outdated entry removed
		assert.NotContains(t, rawStr, "GO-2025-0002")
		assert.NotContains(t, rawStr, "# Vuln two summary")
		// kept entry with updated comment
		assert.Contains(t, rawStr, "# Vuln three updated summary")
		assert.Contains(t, rawStr, "# Fixed in: pkg3@v3.1.0")
		// new entry with comment
		assert.Contains(t, rawStr, "# Brand new vuln four")
		assert.Contains(t, rawStr, "# Found in: pkg4@v4.0.0")
		assert.Contains(t, rawStr, "# Fixed in: pkg4@v4.0.1")
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
