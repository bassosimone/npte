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

// TestAddRoute_dryRunSkipsLstat pins the contract that dry-run does not
// consult the filesystem to decide whether the namespace is managed.
func TestAddRoute_dryRunSkipsLstat(t *testing.T) {
	s := testenv.Setup(t)
	lstatCalls := 0
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		lstatCalls++
		return nil, os.ErrNotExist
	}

	require.NoError(t, addRouteMain(context.Background(),
		[]string{"--dry-run", "client", "default", "10.99.0.1"}))

	assert.Equal(t, -1, s.ExitCode, "dry-run must not exit even with no marker on disk")
	assert.Equal(t, 0, lstatCalls, "dry-run must not consult Lstat")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -d -m 0755 /run/npte/netns",
		`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		"ip netns exec client ip route add default via 10.99.0.1",
	})
}

// TestAddRoute_liveRejectsUnmanaged pins the NOPASSWD audit invariant:
// in live mode, add-route refuses to touch a namespace npte does not own.
func TestAddRoute_liveRejectsUnmanaged(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	require.NoError(t, addRouteMain(context.Background(),
		[]string{"client", "default", "10.99.0.1"}))

	assert.Equal(t, 2, s.ExitCode)
	for _, argv := range s.Commands {
		for _, a := range argv {
			assert.NotEqual(t, "ip", a, "ip must not run when ns is unmanaged")
		}
	}
}
