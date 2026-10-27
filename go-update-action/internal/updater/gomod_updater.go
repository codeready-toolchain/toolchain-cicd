package updater

import (
	"os"
	"regexp"
	"strings"
)

var (
	goDirectiveRe        = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)[ \t]*(\r?)$`)
	toolchainDirectiveRe = regexp.MustCompile(`(?m)^toolchain\s+go(\d+\.\d+\.\d+)[ \t]*(\r?)$`)
)

// updateGoMod bumps the Go version used to build the module, without raising the
// minimum version required by its consumers more than necessary:
//   - if there is a `toolchain` directive matching `from`, it is updated (and so is the
//     `go` directive, if it has the same value or if `to` is a new minor version)
//   - otherwise, if there is no `toolchain` directive and the `go` directive is `from`, it is updated
//   - otherwise, if the `go` directive is a language version (eg: `go 1.26`) matching `from`,
//     a `toolchain` directive is added after it (and the language version is updated if `to`
//     is a new minor version)
//
// When `to` is a new minor version, the `go` directive is updated along with the `toolchain`
// directive, so that they do not refer to different minor versions (eg: `go 1.26.4` and
// `toolchain go1.27.3`). A language version (eg: `go 1.26`) is kept as a language version (eg: `go 1.27`).
func updateGoMod(path, rel, from, to string) (*FileChange, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	old := string(content)
	minorUpgrade := languageVersion(from) != languageVersion(to)
	toolchain := toolchainDirectiveRe.FindStringSubmatch(old)
	updated := goDirectiveRe.ReplaceAllStringFunc(old, func(match string) string {
		sub := goDirectiveRe.FindStringSubmatch(match)
		cr := sub[2] // preserve CRLF line endings
		switch {
		case sub[1] == from && (toolchain == nil || toolchain[1] == from):
			return "go " + to + cr
		case minorUpgrade && toolchain != nil && toolchain[1] == from:
			if sub[1] == languageVersion(sub[1]) {
				return "go " + languageVersion(to) + cr
			}
			return "go " + to + cr
		case sub[1]+".0" == from && toolchain == nil:
			goVersion := sub[1]
			if minorUpgrade {
				goVersion = languageVersion(to)
			}
			return "go " + goVersion + cr + "\n" + cr + "\ntoolchain go" + to + cr
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

// languageVersion returns the `major.minor` part of the given version (eg: `1.26` for `1.26.5`)
func languageVersion(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}
