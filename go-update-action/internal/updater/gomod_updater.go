package updater

import (
	"os"
	"regexp"
)

var (
	goDirectiveRe        = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)[ \t]*(\r?)$`)
	toolchainDirectiveRe = regexp.MustCompile(`(?m)^toolchain\s+go(\d+\.\d+\.\d+)[ \t]*(\r?)$`)
)

// updateGoMod bumps the Go version used to build the module, without raising the
// minimum version required by its consumers more than necessary:
//   - if there is a `toolchain` directive matching `from`, it is updated (and so is the
//     `go` directive, if it has the same value)
//   - otherwise, if there is no `toolchain` directive and the `go` directive is `from`, it is updated
//   - otherwise, if the `go` directive is a language version (eg: `go 1.26`) matching `from`,
//     a `toolchain` directive is added after it
func updateGoMod(path, rel, from, to string) (*FileChange, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	old := string(content)
	toolchain := toolchainDirectiveRe.FindStringSubmatch(old)
	updated := goDirectiveRe.ReplaceAllStringFunc(old, func(match string) string {
		sub := goDirectiveRe.FindStringSubmatch(match)
		cr := sub[2] // preserve CRLF line endings
		switch {
		case sub[1] == from && (toolchain == nil || toolchain[1] == from):
			return "go " + to + cr
		case sub[1]+".0" == from && toolchain == nil:
			return match + "\n" + cr + "\ntoolchain go" + to + cr
		default:
			return match
		}
	})
	updated = toolchainDirectiveRe.ReplaceAllStringFunc(updated, func(match string) string {
		sub := toolchainDirectiveRe.FindStringSubmatch(match)
		if sub[1] != from {
			return match
		}
		return "toolchain go" + to + sub[2]
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
