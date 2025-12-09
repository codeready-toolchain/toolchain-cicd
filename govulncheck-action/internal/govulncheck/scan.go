package govulncheck

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/codeready-toolchain/toolchain-cicd/govulncheck-action/internal/configuration"
	"golang.org/x/vuln/scan"
)

func Scan(ctx context.Context, logger *slog.Logger, scan ScanFunc, path string, config configuration.Configuration) ([]*Vulnerability, []*configuration.Vulnerability, error) {
	rawReport, err := scan(ctx, logger, path)
	if err != nil {
		return nil, nil, err
	}
	// get the vulns from the report
	vulns, err := getVulnerabilities(rawReport)
	if err != nil {
		return nil, nil, err
	}

	// remove ignored vulnerabilities
	return pruneIgnoredVulns(logger, vulns, config.IgnoredVulnerabilities), listOutdatedVulns(vulns, config.IgnoredVulnerabilities), nil
}

type ScanFunc func(ctx context.Context, logger *slog.Logger, path string) ([]byte, error)

func DefaultScan(stderr io.Writer) ScanFunc {
	return func(ctx context.Context, logger *slog.Logger, path string) ([]byte, error) {
		// check that the path exists
		logger.Info("scanning for vulnerabilities", "path", path)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("invalid scan path: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("path is not a directory: %w", err)
		}
		if logger.Enabled(ctx, slog.LevelDebug) {
			files, err := os.ReadDir(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read directory: %w", err)
			}
			for _, file := range files {
				logger.Debug(file.Name())
			}
		}
		c := scan.Command(ctx, "-C", path, "-format", "json", "./...")
		stdout := &bytes.Buffer{}
		c.Stdout = stdout
		c.Stderr = stderr
		if err := c.Start(); err != nil {
			return nil, fmt.Errorf("failed to start golang/govulncheck: %w", err)
		}
		if err := c.Wait(); err != nil {
			fmt.Fprintf(stderr, "%s", stdout.String())
			return nil, fmt.Errorf("failed while running golang/govulncheck: %w", err)
		}
		return stdout.Bytes(), nil
	}
}
