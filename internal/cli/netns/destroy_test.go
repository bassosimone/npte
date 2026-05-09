// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"os"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestroy(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run happy path",
		args:     []string{"--dry-run", "client"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"rm -rf /etc/netns/client",
			"ip netns del client",
			"rm -f /run/npte/netns/client",
		},
	}, {
		name:     "rejects bad name",
		args:     []string{"--dry-run", "1bad"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, destroyMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}

// TestDestroy_dryRunSkipsLstat pins the contract that dry-run does not
// consult the filesystem to decide whether the namespace is managed:
// instead it emits a paste-into-shell guard. See registry.RequireManaged.
func TestDestroy_dryRunSkipsLstat(t *testing.T) {
	s := testenv.Setup(t)
	lstatCalls := 0
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		lstatCalls++
		return nil, os.ErrNotExist
	}

	require.NoError(t, destroyMain(context.Background(), []string{"--dry-run", "client"}))

	assert.Equal(t, -1, s.ExitCode, "dry-run must not exit even with no marker on disk")
	assert.Equal(t, 0, lstatCalls, "dry-run must not consult Lstat")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -d -m 0755 /run/npte/netns",
		`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		"rm -rf /etc/netns/client",
		"ip netns del client",
		"rm -f /run/npte/netns/client",
	})
}

// TestDestroy_liveRejectsUnmanaged pins the NOPASSWD audit invariant:
// in live mode, destroy refuses to delete a namespace npte does not own.
func TestDestroy_liveRejectsUnmanaged(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	require.NoError(t, destroyMain(context.Background(), []string{"client"}))

	assert.Equal(t, 2, s.ExitCode)
	for _, argv := range s.Commands {
		for _, a := range argv {
			assert.NotEqual(t, "ip", a, "ip must not run when ns is unmanaged")
			assert.NotEqual(t, "rm", a, "rm must not run when ns is unmanaged")
		}
	}
}
