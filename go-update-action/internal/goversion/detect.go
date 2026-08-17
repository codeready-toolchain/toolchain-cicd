package goversion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultAPIURL = "https://go.dev/dl/?mode=json"

type Release struct {
	Version Version
	Files   []File
}

type File struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Kind     string `json:"kind"`
}

type apiRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Files   []File `json:"files"`
}

func FetchLatestReleases(ctx context.Context) ([]Release, error) {
	return FetchLatestReleasesFrom(ctx, defaultAPIURL)
}

func FetchLatestReleasesFrom(ctx context.Context, url string) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Go releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from Go downloads API", resp.StatusCode)
	}
	var apiReleases []apiRelease
	if err := json.NewDecoder(resp.Body).Decode(&apiReleases); err != nil {
		return nil, fmt.Errorf("decoding Go releases: %w", err)
	}
	var releases []Release
	for _, ar := range apiReleases {
		if !ar.Stable {
			continue
		}
		v, err := Parse(ar.Version)
		if err != nil {
			continue
		}
		releases = append(releases, Release{Version: v, Files: ar.Files})
	}
	return releases, nil
}

func FindPatchUpdate(current Version, releases []Release) *Version {
	var best *Version
	for _, r := range releases {
		v := r.Version
		if !v.SameMinor(current) {
			continue
		}
		if v.Compare(current) <= 0 {
			continue
		}
		if best == nil || v.Compare(*best) > 0 {
			vCopy := v
			best = &vCopy
		}
	}
	return best
}

func FindMinorUpdate(current Version, releases []Release) *Version {
	var best *Version
	for _, r := range releases {
		v := r.Version
		if v.Major != current.Major {
			continue
		}
		if v.Minor <= current.Minor {
			continue
		}
		if best == nil || v.Compare(*best) > 0 {
			vCopy := v
			best = &vCopy
		}
	}
	return best
}
