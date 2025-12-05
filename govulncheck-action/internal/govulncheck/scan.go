package govulncheck

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/codeready-toolchain/toolchain-cicd/govulncheck-action/internal/configuration"
	"golang.org/x/vuln/scan"
)

func Scan(ctx context.Context, logger *log.Logger, scan ScanFunc, config configuration.Configuration, path string) ([]*Vulnerability, []*configuration.Vulnerability, error) {
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

type ScanFunc func(ctx context.Context, logger *log.Logger, path string) ([]byte, error)

var DefaultScan ScanFunc = func(ctx context.Context, logger *log.Logger, path string) ([]byte, error) {
	// check that the path exists
	_, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	c := scan.Command(ctx, "-C", path, "-format", "json", "./...")
	out := &bytes.Buffer{}
	c.Stdout = out
	c.Stderr = logger.Writer()
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("failed to start golang/govulncheck: %w", err)
	}
	if err := c.Wait(); err != nil {
		return nil, fmt.Errorf("failed while running golang/govulncheck: %w", err)
	}
	return out.Bytes(), nil
}
