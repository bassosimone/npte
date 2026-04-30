// SPDX-License-Identifier: GPL-3.0-or-later

package netem

import (
	"context"
	"os"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClear(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run happy path",
		args:     []string{"--dry-run", "client", "if-router"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client tc qdisc del dev if-router root || true",
		},
	}, {
		name:     "rejects bad ns",
		args:     []string{"--dry-run", "1bad", "if-router"},
		wantExit: 2,
	}, {
		name:     "rejects bad iface",
		args:     []string{"--dry-run", "client", "bad iface"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, clearMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}

// TestClear_dryRunSkipsStat pins the contract that dry-run does not
// consult the filesystem to decide whether the namespace is managed.
func TestClear_dryRunSkipsStat(t *testing.T) {
	s := testenv.Setup(t)
	statCalls := 0
	testable.Env.Stat = func(string) (os.FileInfo, error) {
		statCalls++
		return nil, os.ErrNotExist
	}

	require.NoError(t, clearMain(context.Background(),
		[]string{"--dry-run", "client", "if-router"}))

	assert.Equal(t, -1, s.ExitCode, "dry-run must not exit even with no marker on disk")
	assert.Equal(t, 0, statCalls, "dry-run must not consult Stat")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -d -m 0755 /run/npte/netns",
		`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		"ip netns exec client tc qdisc del dev if-router root || true",
	})
}
