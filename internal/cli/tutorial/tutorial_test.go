// SPDX-License-Identifier: GPL-3.0-or-later

package tutorial

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain_Tutorial(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantExit    int
		wantNonZero bool // expect stdout to have content
	}{{
		name:        "no args renders TOC",
		args:        nil,
		wantExit:    -1,
		wantNonZero: true,
	}, {
		name:        "all renders concatenation",
		args:        []string{"all"},
		wantExit:    -1,
		wantNonZero: true,
	}, {
		name:        "known slug renders one chapter",
		args:        []string{"netns-basics"},
		wantExit:    -1,
		wantNonZero: true,
	}, {
		name:     "unknown slug exits 1",
		args:     []string{"does-not-exist"},
		wantExit: 1,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, Main(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			if tc.wantNonZero {
				assert.NotEmpty(t, s.Stdout.String())
			}
		})
	}
}

func TestLoadChapters(t *testing.T) {
	chapters := loadChapters()
	require.NotEmpty(t, chapters)

	slugs := make([]string, len(chapters))
	for i, ch := range chapters {
		slugs[i] = ch.slug
		assert.NotEmpty(t, ch.title, "chapter %q has empty title", ch.slug)
		assert.NotEmpty(t, ch.body, "chapter %q has empty body", ch.slug)
		assert.NotContains(t, ch.slug, "/", "slug should not contain path separator")
		assert.False(t, strings.HasPrefix(ch.slug, "0"),
			"slug %q still has numeric prefix", ch.slug)
	}
	assert.Contains(t, slugs, "netns-basics")
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		fallback string
		want     string
	}{
		{"first heading wins", "# First\n# Second\n", "fb", "First"},
		{"no heading uses fallback", "no heading here\n", "fb", "fb"},
		{"heading with trailing space", "#   Spaced   \n", "fb", "Spaced"},
		{"empty body uses fallback", "", "fb", "fb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, extractTitle(tc.body, tc.fallback))
		})
	}
}

func TestBuildTOC(t *testing.T) {
	t.Run("with chapters", func(t *testing.T) {
		got := buildTOC([]chapter{
			{slug: "alpha", title: "Alpha"},
			{slug: "beta", title: "Beta"},
		})
		assert.Contains(t, got, "# npte tutorial")
		assert.Contains(t, got, "## Chapters")
		assert.Contains(t, got, "`alpha` — Alpha")
		assert.Contains(t, got, "`beta` — Beta")
	})

	t.Run("no chapters falls back", func(t *testing.T) {
		got := buildTOC(nil)
		assert.Contains(t, got, "No chapters are currently embedded.")
		assert.NotContains(t, got, "## Chapters")
	})
}

func TestConcatChapters(t *testing.T) {
	got := concatChapters([]chapter{
		{slug: "a", body: "AAA"},
		{slug: "b", body: "BBB"},
	})
	assert.Equal(t, "AAA\n\n---\n\nBBB", got)

	assert.Empty(t, concatChapters(nil))
}

func TestRender(t *testing.T) {
	// glamour with WithAutoStyle falls back to a plain style in non-TTY,
	// so it should produce non-empty output for any non-empty input. We
	// don't assert exact content (ANSI/styling may shift across versions).
	var buf bytes.Buffer
	render(&buf, "# Hi\n\nhello\n")
	assert.NotEmpty(t, buf.String())
	assert.Contains(t, buf.String(), "Hi")
}
