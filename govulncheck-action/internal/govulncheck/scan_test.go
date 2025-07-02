package govulncheck_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/codeready-toolchain/toolchain-cicd/govulncheck-action/internal/configuration"
	"github.com/codeready-toolchain/toolchain-cicd/govulncheck-action/internal/govulncheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan(t *testing.T) {

	t.Run("no vuln found", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *log.Logger, _ string) ([]byte, error) {
			return nil, nil
		}
		logger := log.Default()
		config := configuration.Configuration{}
		path := "./..."

		// when
		vulns, err := govulncheck.Scan(context.Background(), logger, scan, config, path)

		// then
		require.NoError(t, err)
		assert.Empty(t, vulns)
	})

	t.Run("2 vulns found", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *log.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := log.Default()
		config := configuration.Configuration{}
		path := "./..."

		// when
		vulns, err := govulncheck.Scan(context.Background(), logger, scan, config, path)

		// then
		require.NoError(t, err)
		assert.Len(t, vulns, 2)
	})

	t.Run("2 vulns found and 1 ignored", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *log.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := log.Default()
		config := configuration.Configuration{
			IgnoredVulnerabilities: []*configuration.Vulnerability{
				{
					ID:           "GO-2025-3563",
					SilenceUntil: time.Now().Add(24 * time.Hour),
				},
			},
		}
		path := "./..."

		// when
		vulns, err := govulncheck.Scan(context.Background(), logger, scan, config, path)

		// then
		require.NoError(t, err)
		assert.Len(t, vulns, 1)
	})

	t.Run("2 vulns found and 1 ignored and 1 expired", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *log.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := log.Default()
		config := configuration.Configuration{
			IgnoredVulnerabilities: []*configuration.Vulnerability{
				{
					ID:           "GO-2025-3563",
					SilenceUntil: time.Now().Add(24 * time.Hour),
				},
				{
					ID:           "GO-2025-3547",
					SilenceUntil: time.Now().Add(-24 * time.Hour),
				},
			},
		}
		path := "./..."

		// when
		vulns, err := govulncheck.Scan(context.Background(), logger, scan, config, path)

		// then
		require.NoError(t, err)
		assert.Len(t, vulns, 1)
	})

	t.Run("2 vulns found and 2 ignored", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *log.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := log.Default()
		config := configuration.Configuration{
			IgnoredVulnerabilities: []*configuration.Vulnerability{
				{
					ID:           "GO-2025-3563",
					SilenceUntil: time.Now().Add(24 * time.Hour),
				},
				{
					ID:           "GO-2025-3547",
					SilenceUntil: time.Now().Add(24 * time.Hour),
				},
			},
		}
		path := "./..."

		// when
		vulns, err := govulncheck.Scan(context.Background(), logger, scan, config, path)

		// then
		require.NoError(t, err)
		assert.Empty(t, vulns)
	})

}
