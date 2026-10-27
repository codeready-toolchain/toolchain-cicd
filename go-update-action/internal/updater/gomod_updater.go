package updater

import (
	"os"
	"regexp"

	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/goversion"
)

// the directives may have a trailing comment (eg: `go 1.26 // comment`), which is captured to be preserved
var (
	goDirectiveRe        = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)([ \t]*//[^\r\n]*)?[ \t]*(\r?)$`)
	toolchainDirectiveRe = regexp.MustCompile(`(?m)^toolchain\s+go(\d+\.\d+\.\d+)([ \t]*//[^\r\n]*)?[ \t]*(\r?)$`)
)

// updateGoMod sets the `go` directive to the language version of `to` (eg: `go 1.27`) and the
// `toolchain` directive to `to` (eg: `toolchain go1.27.3`), adding the `toolchain` directive if
// it is missing. The `go` directive is not set to `to`, as `go mod tidy` would then remove the
// `toolchain` directive, which would be redundant.
//
// The version of the module is read from its `toolchain` directive if present, from its `go`
// directive otherwise (where `go 1.26` stands for `1.26.0`). The file is only updated if the
// module is on a version older than `to`, whatever its minor version.
func updateGoMod(path, rel, to string) (*FileChange, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	old := string(content)
	goDirective := goDirectiveRe.FindStringSubmatch(old)
	if goDirective == nil {
		return nil, nil
	}
	toolchain := toolchainDirectiveRe.FindStringSubmatch(old)
	current := goDirective[1]
	if toolchain != nil {
		current = toolchain[1]
	}
	currentVersion, err := goversion.Parse(current)
	if err != nil {
		return nil, err
	}
	target, err := goversion.Parse(to)
	if err != nil {
		return nil, err
	}
	if currentVersion.Compare(target) >= 0 {
		return nil, nil
	}

	comment, cr := goDirective[2], goDirective[3] // preserve trailing comment and CRLF line endings
	var updated string
	if toolchain != nil {
		updated = goDirectiveRe.ReplaceAllLiteralString(old, "go "+target.MinorString()+comment+cr)
		updated = toolchainDirectiveRe.ReplaceAllLiteralString(updated, "toolchain go"+target.String()+toolchain[2]+toolchain[3])
	} else {
		updated = goDirectiveRe.ReplaceAllLiteralString(old, "go "+target.MinorString()+comment+cr+"\n"+cr+"\ntoolchain go"+target.String()+cr)
	}

	if updated == old {
		return nil, nil
	}

	return &FileChange{
		Path:       rel,
		OldContent: old,
		NewContent: updated,
	}, nil
}
