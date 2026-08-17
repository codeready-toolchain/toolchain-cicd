package updater

import (
	"os"
	"regexp"
)

var goDirectiveRe = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+\.\d+)\s*$`)

func updateGoMod(path, rel, from, to string) (*FileChange, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	old := string(content)
	updated := goDirectiveRe.ReplaceAllStringFunc(old, func(match string) string {
		sub := goDirectiveRe.FindStringSubmatch(match)
		if len(sub) < 2 || sub[1] != from {
			return match
		}
		return "go " + to
	})

	if updated == old {
		return nil, nil
	}

	return &FileChange{
		Path:       rel,
		OldContent: old,
		NewContent: updated,
	}, nil
}
