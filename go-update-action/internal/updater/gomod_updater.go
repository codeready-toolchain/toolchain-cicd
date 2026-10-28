package updater

import (
	"os"
	"regexp"
	"strings"
)

var (
	goDirectiveRe        = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)[ \t]*(\r?)$`)
	toolchainDirectiveRe = regexp.MustCompile(`(?m)^toolchain\s+go(\d+\.\d+\.\d+)[ \t]*(\r?)$`)
	// matches any `toolchain` directive, including the ones that are not updated (eg: `toolchain default`)
	anyToolchainDirectiveRe = regexp.MustCompile(`(?m)^toolchain\s`)
)

// updateGoMod bumps the Go version used to build the module, without raising the
// minimum version required by its consumers more than necessary:
//   - if there is a `toolchain` directive matching `from`, it is updated (and so is the
//     `go` directive, if it has the same value and `to` is not a new minor version)
//   - otherwise, if there is no `toolchain` directive and the `go` directive is `from`, it is updated
//     if `to` is not a new minor version
//   - otherwise, if the `go` directive is a language version (eg: `go 1.26`) matching `from`, or
//     if `to` is a new minor version, it is kept and a `toolchain` directive is added after it
//     (unless there is already a `toolchain` directive with another value, eg: `toolchain default`)
//
// In other words, the `go` directive is never updated when `to` is a new minor version
// (eg: `go 1.26.8` becomes `go 1.26.8` and `toolchain go1.27.3`).
func updateGoMod(path, rel, from, to string) (*FileChange, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	old := string(content)
	minorUpgrade := languageVersion(from) != languageVersion(to)
	toolchain := toolchainDirectiveRe.FindStringSubmatch(old)
	hasToolchain := anyToolchainDirectiveRe.MatchString(old)
	updated := goDirectiveRe.ReplaceAllStringFunc(old, func(match string) string {
		sub := goDirectiveRe.FindStringSubmatch(match)
		cr := sub[2] // preserve CRLF line endings
		switch {
		case sub[1] == from && !minorUpgrade && (toolchain == nil || toolchain[1] == from):
			return "go " + to + cr
		case !hasToolchain && (sub[1]+".0" == from || (sub[1] == from && minorUpgrade)):
			return "go " + sub[1] + cr + "\n" + cr + "\ntoolchain go" + to + cr
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
