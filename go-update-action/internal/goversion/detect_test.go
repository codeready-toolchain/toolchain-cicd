package goversion_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/goversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchLatestReleases(t *testing.T) {
	apiResponse := []map[string]any{
		{
			"version": "go1.27.2",
			"stable":  true,
			"files":   []map[string]any{},
		},
		{
			"version": "go1.26.6",
			"stable":  true,
			"files":   []map[string]any{},
		},
		{
			"version": "go1.28rc1",
			"stable":  false,
			"files":   []map[string]any{},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse) //nolint:errcheck
	}))
	defer srv.Close()

	releases, err := goversion.FetchLatestReleasesFrom(context.Background(), srv.URL)
	require.NoError(t, err)
	require.Len(t, releases, 2)
	assert.Equal(t, goversion.Version{Major: 1, Minor: 27, Patch: 2}, releases[0].Version)
	assert.Equal(t, goversion.Version{Major: 1, Minor: 26, Patch: 6}, releases[1].Version)
}

func TestFindPatchUpdate(t *testing.T) {
	releases := []goversion.Release{
		{Version: goversion.Version{Major: 1, Minor: 27, Patch: 2}},
		{Version: goversion.Version{Major: 1, Minor: 26, Patch: 3}},
		{Version: goversion.Version{Major: 1, Minor: 25, Patch: 5}},
	}

	t.Run("update available", func(t *testing.T) {
		current := goversion.Version{Major: 1, Minor: 26, Patch: 1}
		got := goversion.FindPatchUpdate(current, releases)
		require.NotNil(t, got)
		assert.Equal(t, goversion.Version{Major: 1, Minor: 26, Patch: 3}, *got)
	})

	t.Run("already on latest patch", func(t *testing.T) {
		current := goversion.Version{Major: 1, Minor: 26, Patch: 3}
		got := goversion.FindPatchUpdate(current, releases)
		assert.Nil(t, got)
	})

	t.Run("no matching minor line", func(t *testing.T) {
		current := goversion.Version{Major: 1, Minor: 24, Patch: 0}
		got := goversion.FindPatchUpdate(current, releases)
		assert.Nil(t, got)
	})
}

func TestFindMinorUpdate(t *testing.T) {
	releases := []goversion.Release{
		{Version: goversion.Version{Major: 1, Minor: 27, Patch: 2}},
		{Version: goversion.Version{Major: 1, Minor: 26, Patch: 3}},
	}

	t.Run("update available", func(t *testing.T) {
		current := goversion.Version{Major: 1, Minor: 26, Patch: 1}
		got := goversion.FindMinorUpdate(current, releases)
		require.NotNil(t, got)
		assert.Equal(t, goversion.Version{Major: 1, Minor: 27, Patch: 2}, *got)
	})

	t.Run("already on latest minor", func(t *testing.T) {
		current := goversion.Version{Major: 1, Minor: 27, Patch: 0}
		got := goversion.FindMinorUpdate(current, releases)
		assert.Nil(t, got)
	})
}
