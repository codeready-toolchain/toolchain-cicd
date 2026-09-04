package goversion_test

import (
	"testing"

	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/goversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    goversion.Version
		wantErr bool
	}{
		{
			name:  "full version",
			input: "1.26.1",
			want:  goversion.Version{Major: 1, Minor: 26, Patch: 1},
		},
		{
			name:  "with go prefix",
			input: "go1.26.1",
			want:  goversion.Version{Major: 1, Minor: 26, Patch: 1},
		},
		{
			name:  "minor only",
			input: "1.26",
			want:  goversion.Version{Major: 1, Minor: 26, Patch: 0},
		},
		{
			name:  "minor only with go prefix",
			input: "go1.26",
			want:  goversion.Version{Major: 1, Minor: 26, Patch: 0},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "single number",
			input:   "1",
			wantErr: true,
		},
		{
			name:    "non-numeric",
			input:   "abc.def",
			wantErr: true,
		},
		{
			name:    "too many parts",
			input:   "1.26.1.0",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := goversion.Parse(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersion_String(t *testing.T) {
	v := goversion.Version{Major: 1, Minor: 26, Patch: 3}
	assert.Equal(t, "1.26.3", v.String())
}

func TestVersion_MinorString(t *testing.T) {
	v := goversion.Version{Major: 1, Minor: 26, Patch: 3}
	assert.Equal(t, "1.26", v.MinorString())
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b goversion.Version
		want int
	}{
		{
			name: "equal",
			a:    goversion.Version{Major: 1, Minor: 26, Patch: 1},
			b:    goversion.Version{Major: 1, Minor: 26, Patch: 1},
			want: 0,
		},
		{
			name: "patch less",
			a:    goversion.Version{Major: 1, Minor: 26, Patch: 1},
			b:    goversion.Version{Major: 1, Minor: 26, Patch: 3},
			want: -1,
		},
		{
			name: "patch greater",
			a:    goversion.Version{Major: 1, Minor: 26, Patch: 3},
			b:    goversion.Version{Major: 1, Minor: 26, Patch: 1},
			want: 1,
		},
		{
			name: "minor less",
			a:    goversion.Version{Major: 1, Minor: 25, Patch: 5},
			b:    goversion.Version{Major: 1, Minor: 26, Patch: 1},
			want: -1,
		},
		{
			name: "minor greater",
			a:    goversion.Version{Major: 1, Minor: 27, Patch: 0},
			b:    goversion.Version{Major: 1, Minor: 26, Patch: 9},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.a.Compare(tt.b))
		})
	}
}

func TestSameMinor(t *testing.T) {
	a := goversion.Version{Major: 1, Minor: 26, Patch: 1}
	b := goversion.Version{Major: 1, Minor: 26, Patch: 5}
	c := goversion.Version{Major: 1, Minor: 27, Patch: 0}

	assert.True(t, a.SameMinor(b))
	assert.False(t, a.SameMinor(c))
}
