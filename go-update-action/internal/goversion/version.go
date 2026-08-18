package goversion

import (
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func Parse(s string) (Version, error) {
	s = strings.TrimPrefix(s, "go")
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, fmt.Errorf("invalid Go version: %q", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version in %q: %w", s, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version in %q: %w", s, err)
	}
	patch := 0
	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch version in %q: %w", s, err)
		}
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) MinorString() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		return cmp(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return cmp(v.Minor, other.Minor)
	}
	return cmp(v.Patch, other.Patch)
}

func (v Version) SameMinor(other Version) bool {
	return v.Major == other.Major && v.Minor == other.Minor
}

func cmp(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
