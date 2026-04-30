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

func TestAddRoute(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run default route",
		args:     []string{"--dry-run", "client", "default", "10.99.0.1"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client ip route add default via 10.99.0.1",
		},
	}, {
		name:     "dry-run cidr route",
		args:     []string{"--dry-run", "client", "10.0.0.0/8", "10.99.0.1"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client ip route add 10.0.0.0/8 via 10.99.0.1",
		},
	}, {
		name:     "rejects bad ns",
		args:     []string{"--dry-run", "1bad", "default", "10.99.0.1"},
		wantExit: 2,
	}, {
		name:     "rejects bad dest",
		args:     []string{"--dry-run", "client", "garbage", "10.99.0.1"},
		wantExit: 2,
	}, {
		name:     "rejects bad via",
		args:     []string{"--dry-run", "client", "default", "not-an-ip"},
		wantExit: 2,
	}, {
		name:     "rejects family mismatch",
		args:     []string{"--dry-run", "client", "10.0.0.0/8", "fc00::1"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, addRouteMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}

// TestAddRoute_dryRunSkipsStat pins the contract that dry-run does not
// consult the filesystem to decide whether the namespace is managed.
func TestAddRoute_dryRunSkipsStat(t *testing.T) {
	s := testenv.Setup(t)
	statCalls := 0
	testable.Env.Stat = func(string) (os.FileInfo, error) {
		statCalls++
		return nil, os.ErrNotExist
	}

	require.NoError(t, addRouteMain(context.Background(),
		[]string{"--dry-run", "client", "default", "10.99.0.1"}))

	assert.Equal(t, -1, s.ExitCode, "dry-run must not exit even with no marker on disk")
	assert.Equal(t, 0, statCalls, "dry-run must not consult Stat")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -d -m 0755 /run/npte/netns",
		`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		"ip netns exec client ip route add default via 10.99.0.1",
	})
}
