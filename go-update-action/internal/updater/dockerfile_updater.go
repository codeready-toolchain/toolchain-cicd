package updater

import (
	"os"
	"regexp"
	"strings"
)

var (
	fromGolangRe = regexp.MustCompile(
		`(?m)^(FROM\s+golang:)(\d+\.\d+\.\d+)(.*)\s*$`)

	goVersionArgRe = regexp.MustCompile(
		`(?m)^((ARG|ENV)\s+(?:GO_VERSION|GOLANG_VERSION|GO_VER)\s*=\s*)(\d+\.\d+\.\d+)(\s*)$`)
)

func updateDockerfile(path, rel, from, to string) (*FileChange, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	old := string(content)
	updated := old

	updated = fromGolangRe.ReplaceAllStringFunc(updated, func(match string) string {
		sub := fromGolangRe.FindStringSubmatch(match)
		if len(sub) < 4 || sub[2] != from {
			return match
		}
		return sub[1] + to + sub[3]
	})

	updated = goVersionArgRe.ReplaceAllStringFunc(updated, func(match string) string {
		sub := goVersionArgRe.FindStringSubmatch(match)
		if len(sub) < 5 || sub[3] != from {
			return match
		}
		return sub[1] + to + sub[4]
	})

	updated = strings.TrimRight(updated, "\n") + "\n"
	old = strings.TrimRight(old, "\n") + "\n"

	if updated == old {
		return nil, nil
	}

	return &FileChange{
		Path:       rel,
		OldContent: old,
		NewContent: updated,
	}, nil
}
