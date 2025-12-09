package govulncheck_test

import (
	"context"
	"log/slog"
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
		scan := func(ctx context.Context, logger *slog.Logger, _ string) ([]byte, error) {
			return nil, nil
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		config := configuration.Configuration{}
		path := "./..."

		// when
		vulns, outdatedVulns, err := govulncheck.Scan(context.Background(), logger, scan, path, config)

		// then
		require.NoError(t, err)
		assert.Empty(t, vulns)
		assert.Empty(t, outdatedVulns)
	})

	t.Run("2 vulns found", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *slog.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		config := configuration.Configuration{}
		path := "./..."

		// when
		vulns, outdatedVulns, err := govulncheck.Scan(context.Background(), logger, scan, path, config)

		// then
		require.NoError(t, err)
		assert.Len(t, vulns, 2)
		assert.Empty(t, outdatedVulns)
	})

	t.Run("2 vulns found and 1 ignored", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *slog.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
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
		vulns, outdatedVulns, err := govulncheck.Scan(context.Background(), logger, scan, path, config)

		// then
		require.NoError(t, err)
		assert.Len(t, vulns, 1)
		assert.Empty(t, outdatedVulns)
	})

	t.Run("2 vulns found and 1 ignored and 1 expired", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *slog.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
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
		vulns, outdatedVulns, err := govulncheck.Scan(context.Background(), logger, scan, path, config)

		// then
		require.NoError(t, err)
		assert.Len(t, vulns, 1)
		assert.Empty(t, outdatedVulns)
	})

	t.Run("2 vulns found and 2 ignored", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *slog.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
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
		vulns, outdatedVulns, err := govulncheck.Scan(context.Background(), logger, scan, path, config)

		// then
		require.NoError(t, err)
		assert.Empty(t, vulns)
		assert.Empty(t, outdatedVulns)
	})

	t.Run("2 vulns found and 2 ignored and 1 outdated", func(t *testing.T) {
		// given
		scan := func(ctx context.Context, logger *slog.Logger, _ string) ([]byte, error) {
			return os.ReadFile("../testdata/valid_report.json")
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
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
				{
					ID:           "GO-0000-0000", // non-existing vulnerability
					SilenceUntil: time.Now().Add(24 * time.Hour),
				},
			},
		}
		path := "./..."

		// when
		vulns, outdatedVulns, err := govulncheck.Scan(context.Background(), logger, scan, path, config)

		// then
		require.NoError(t, err)
		assert.Empty(t, vulns)
		assert.Len(t, outdatedVulns, 1)
		assert.Equal(t, "GO-0000-0000", outdatedVulns[0].ID)
	})

}
