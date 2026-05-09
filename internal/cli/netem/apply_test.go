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

func TestApply(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run delay only",
		args:     []string{"--dry-run", "--delay", "10ms", "client", "if-router"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client tc qdisc add dev if-router root handle 1: netem delay 10ms",
		},
	}, {
		name:     "dry-run with cake child",
		args:     []string{"--dry-run", "--delay", "10ms", "--child", "cake", "--cake-bandwidth", "30mbit", "client", "if-router"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client tc qdisc add dev if-router root handle 1: netem delay 10ms",
			"ip netns exec client tc qdisc add dev if-router parent 1: handle 2: cake bandwidth 30mbit",
		},
	}, {
		name:     "rejects bad ns",
		args:     []string{"--dry-run", "--delay", "10ms", "1bad", "if-router"},
		wantExit: 2,
	}, {
		name:     "rejects bad iface",
		args:     []string{"--dry-run", "--delay", "10ms", "client", "bad/iface"},
		wantExit: 2,
	}, {
		name:     "rejects no knobs",
		args:     []string{"--dry-run", "client", "if-router"},
		wantExit: 2,
	}, {
		name:     "rejects bad delay",
		args:     []string{"--dry-run", "--delay", "garbage", "client", "if-router"},
		wantExit: 2,
	}, {
		// Child-kind validation runs before any kernel command is emitted, so on a
		// bad --child value neither the root nor child netem line appears.
		name:     "rejects bad child kind",
		args:     []string{"--dry-run", "--delay", "10ms", "--child", "bogus", "client", "if-router"},
		wantExit: 2,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, applyMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}

// TestApply_dryRunSkipsLstat pins the contract that dry-run does not
// consult the filesystem to decide whether the namespace is managed.
func TestApply_dryRunSkipsLstat(t *testing.T) {
	s := testenv.Setup(t)
	lstatCalls := 0
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		lstatCalls++
		return nil, os.ErrNotExist
	}

	require.NoError(t, applyMain(context.Background(),
		[]string{"--dry-run", "--delay", "10ms", "client", "if-router"}))

	assert.Equal(t, -1, s.ExitCode, "dry-run must not exit even with no marker on disk")
	assert.Equal(t, 0, lstatCalls, "dry-run must not consult Lstat")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -d -m 0755 /run/npte/netns",
		`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		"ip netns exec client tc qdisc add dev if-router root handle 1: netem delay 10ms",
	})
}
