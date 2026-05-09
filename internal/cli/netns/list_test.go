// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	// list does not support --dry-run; with the empty ReadDir stub from
	// testenv.Setup it just produces empty stdout.
	s := testenv.Setup(t)
	require.NoError(t, listMain(context.Background(), nil))
	assert.Equal(t, -1, s.ExitCode)
	assert.Equal(t, "", s.Stdout.String())
}

// fakeDirEntry implements [fs.DirEntry] for a regular-file marker. Only
// Name() and Type() are exercised by registry.List; Info() is unused but
// implemented for completeness.
type fakeDirEntry string

func (e fakeDirEntry) Name() string               { return string(e) }
func (fakeDirEntry) IsDir() bool                  { return false }
func (fakeDirEntry) Type() fs.FileMode            { return 0 } // regular file
func (e fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

// TestList_nonEmpty exercises the loop body that prints each managed
// netns name to stdout. ReadDir is stubbed to return three markers,
// one of which has a malformed name and must be silently skipped (per
// registry.List's validate.NetnsName filter).
func TestList_nonEmpty(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.ReadDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			fakeDirEntry("router"),
			fakeDirEntry("1bad"), // fails validate.NetnsName, must be skipped
			fakeDirEntry("client"),
		}, nil
	}

	require.NoError(t, listMain(context.Background(), nil))

	assert.Equal(t, -1, s.ExitCode)
	// registry.List sorts the names; "1bad" is filtered out.
	assert.Equal(t, "client\nrouter\n", s.Stdout.String())
}

// TestList_readDirError pins the error path: when ReadDir fails for a
// reason other than ErrNotExist (which registry.List swallows), listMain
// logs and exits 2.
func TestList_readDirError(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.ReadDir = func(string) ([]os.DirEntry, error) {
		return nil, errors.New("boom")
	}

	require.NoError(t, listMain(context.Background(), nil))

	assert.Equal(t, 2, s.ExitCode)
	assert.Equal(t, "", s.Stdout.String())
}
