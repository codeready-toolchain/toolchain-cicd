package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileChange struct {
	Path       string // relative to repo root
	OldContent string
	NewContent string
}

type SuggestionChange struct {
	Path       string
	OldContent string
	NewContent string
}

type Result struct {
	Changes     []FileChange
	Suggestions []SuggestionChange
}

func ScanAndUpdate(root string, from, to string, paths []string, excludes []string) (*Result, error) {
	result := &Result{}
	seen := make(map[string]bool)

	for _, p := range paths {
		scanRoot := filepath.Join(root, p)
		info, err := os.Stat(scanRoot)
		if err != nil {
			return nil, fmt.Errorf("path %q: %w", p, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("path %q is not a directory", p)
		}

		err = filepath.WalkDir(scanRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			if seen[rel] {
				return nil
			}

			if isExcluded(rel, excludes) {
				return nil
			}

			name := d.Name()

			switch {
			case name == "go.mod":
				change, err := updateGoMod(path, rel, from, to)
				if err != nil {
					return err
				}
				if change != nil {
					seen[rel] = true
					result.Changes = append(result.Changes, *change)
				}

			case isDockerfile(name):
				change, err := updateDockerfile(path, rel, from, to)
				if err != nil {
					return err
				}
				if change != nil {
					seen[rel] = true
					result.Changes = append(result.Changes, *change)
				}

			case isWorkflowFile(rel):
				change, suggestion, err := updateWorkflow(path, rel, from, to)
				if err != nil {
					return err
				}
				if change != nil {
					seen[rel] = true
					result.Changes = append(result.Changes, *change)
				}
				if suggestion != nil {
					result.Suggestions = append(result.Suggestions, *suggestion)
				}
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking %q: %w", p, err)
		}
	}

	return result, nil
}

func isExcluded(rel string, excludes []string) bool {
	for _, pattern := range excludes {
		if matched, _ := filepath.Match(pattern, filepath.Base(rel)); matched {
			return true
		}
		for path := rel; path != "."; path = filepath.Dir(path) {
			if matched, _ := filepath.Match(pattern, path); matched {
				return true
			}
		}
	}
	return false
}

func isDockerfile(name string) bool {
	return strings.HasPrefix(name, "Dockerfile") || strings.HasPrefix(name, "Containerfile")
}

func isWorkflowFile(rel string) bool {
	dir := filepath.Dir(rel)
	return dir == filepath.Join(".github", "workflows") &&
		(strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml"))
}
