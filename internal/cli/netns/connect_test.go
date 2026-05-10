// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run happy path",
		args:     []string{"--dry-run", "client", "router"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			`test -f /run/npte/netns/router || { echo 'npte: router: not managed by npte' >&2; exit 2; }`,
			"ip link add if-router type veth peer name if-client",
			"ip link set if-router netns client",
			"ip link set if-client netns router",
			"ip netns exec client ip link set if-router up",
			"ip netns exec router ip link set if-client up",
		},
	}, {
		name:     "rejects identical endpoints",
		args:     []string{"--dry-run", "client", "client"},
		wantExit: 2,
	}, {
		name:     "rejects bad left",
		args:     []string{"--dry-run", "1bad", "router"},
		wantExit: 2,
	}, {
		name:     "rejects bad right",
		args:     []string{"--dry-run", "client", "1bad"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, connectMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}

// TestConnect_dryRunSkipsLstat is a regression test for a bug where dry-run
// composition (e.g. `npte lab create --dry-run`) aborted at the first
// `netns connect` because RequireManaged stat'd the marker — which never
// existed, since the previous `netns create -n` only printed its install
// command. The fix moved the dry-run branch above the filesystem call and
// made it emit a shell guard instead. This test pins that behaviour: a
// dry-run connect with Lstat → ErrNotExist must succeed and the guard must
// be in the rendered script.
func TestConnect_dryRunSkipsLstat(t *testing.T) {
	s := testenv.Setup(t)
	lstatCalls := 0
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		lstatCalls++
		return nil, os.ErrNotExist
	}

	require.NoError(t, connectMain(context.Background(), []string{"--dry-run", "client", "router"}))

	assert.Equal(t, -1, s.ExitCode, "dry-run must not exit even with no marker on disk")
	assert.Equal(t, 0, lstatCalls, "dry-run must not consult Lstat")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -d -m 0755 /run/npte/netns",
		`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		`test -f /run/npte/netns/router || { echo 'npte: router: not managed by npte' >&2; exit 2; }`,
		"ip link add if-router type veth peer name if-client",
		"ip link set if-router netns client",
		"ip link set if-client netns router",
		"ip netns exec client ip link set if-router up",
		"ip netns exec router ip link set if-client up",
	})
}

// TestConnect_liveRejectsUnmanagedLeft pins the NOPASSWD audit invariant
// for the left positional: in live mode, connect refuses to wire a veth
// into a namespace npte does not own.
func TestConnect_liveRejectsUnmanagedLeft(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.Lstat = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	require.NoError(t, connectMain(context.Background(),
		[]string{"client", "router"}))

	assert.Equal(t, 2, s.ExitCode)
	for _, argv := range s.Commands {
		for _, a := range argv {
			assert.NotEqual(t, "ip", a, "ip must not run when ns is unmanaged")
		}
	}
}

// TestConnect_liveRejectsUnmanagedRight pins the NOPASSWD audit invariant
// for the right positional: even when left is managed, connect must
// refuse if right is not.
func TestConnect_liveRejectsUnmanagedRight(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.Lstat = func(name string) (os.FileInfo, error) {
		// Accept the left marker, refuse the right one.
		if name == "/run/npte/netns/client" {
			return regularFileInfo("client"), nil
		}
		return nil, os.ErrNotExist
	}

	require.NoError(t, connectMain(context.Background(),
		[]string{"client", "router"}))

	assert.Equal(t, 2, s.ExitCode)
	for _, argv := range s.Commands {
		for _, a := range argv {
			assert.NotEqual(t, "ip", a, "ip must not run when ns is unmanaged")
		}
	}
}

// regularFileInfo implements os.FileInfo for a fake regular file.
type regularFileInfo string

func (n regularFileInfo) Name() string     { return string(n) }
func (regularFileInfo) Size() int64        { return 0 }
func (regularFileInfo) Mode() os.FileMode  { return 0o644 }
func (regularFileInfo) ModTime() time.Time { return time.Time{} }
func (regularFileInfo) IsDir() bool        { return false }
func (regularFileInfo) Sys() any           { return nil }
