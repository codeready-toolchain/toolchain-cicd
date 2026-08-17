package updater

import (
	"os"
	"regexp"
)

var goVersionKeyRe = regexp.MustCompile(
	`(?m)^(\s*)(go-version:\s*)(['"]?)(\d+\.\d+\.\d+)(['"]?)(\s*)$`)

func updateWorkflow(path, rel, from, to string) (*FileChange, *SuggestionChange, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	old := string(content)
	var hasHardcodedVersion bool

	updated := goVersionKeyRe.ReplaceAllStringFunc(old, func(match string) string {
		sub := goVersionKeyRe.FindStringSubmatch(match)
		if len(sub) < 7 || sub[4] != from {
			if len(sub) >= 7 && isVersionString(sub[4]) {
				hasHardcodedVersion = true
			}
			return match
		}
		hasHardcodedVersion = true
		return sub[1] + sub[2] + sub[3] + to + sub[5] + sub[6]
	})

	var change *FileChange
	if updated != old {
		change = &FileChange{
			Path:       rel,
			OldContent: old,
			NewContent: updated,
		}
	}

	var suggestion *SuggestionChange
	if hasHardcodedVersion {
		suggested := goVersionKeyRe.ReplaceAllStringFunc(old, func(match string) string {
			sub := goVersionKeyRe.FindStringSubmatch(match)
			if len(sub) < 7 || !isVersionString(sub[4]) {
				return match
			}
			return sub[1] + "go-version-file: 'go.mod'" + sub[6]
		})
		if suggested != old {
			suggestion = &SuggestionChange{
				Path:       rel,
				OldContent: old,
				NewContent: suggested,
			}
		}
	}

	return change, suggestion, nil
}

func isVersionString(s string) bool {
	matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+$`, s)
	return matched
}
