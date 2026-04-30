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

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		sudoUser string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run happy path",
		sudoUser: "alice",
		args:     []string{"--dry-run", "client", "ip", "addr"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client runuser -u alice -- env ip addr",
		},
	}, {
		name:     "dry-run with env vars",
		sudoUser: "alice",
		args:     []string{"--dry-run", "-e", "FOO=bar", "client", "ip", "addr"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
			"ip netns exec client runuser -u alice -- env FOO=bar ip addr",
		},
	}, {
		name:     "rejects bad ns",
		sudoUser: "alice",
		args:     []string{"--dry-run", "1bad", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "rejects missing SUDO_USER",
		sudoUser: "",
		args:     []string{"--dry-run", "client", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "rejects malformed SUDO_USER",
		sudoUser: "1bad",
		args:     []string{"--dry-run", "client", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "rejects bad env var",
		sudoUser: "alice",
		args:     []string{"--dry-run", "-e", "1BAD=x", "client", "ip", "addr"},
		wantExit: 2,
	}, {
		name:     "rejects env without =",
		sudoUser: "alice",
		args:     []string{"--dry-run", "-e", "noequals", "client", "ip", "addr"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			s.SudoUser = tc.sudoUser
			require.NoError(t, runMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}

// TestRun_dryRunSkipsStat pins the contract that dry-run does not consult
// the filesystem to decide whether the namespace is managed.
func TestRun_dryRunSkipsStat(t *testing.T) {
	s := testenv.Setup(t)
	s.SudoUser = "alice"
	statCalls := 0
	testable.Env.Stat = func(string) (os.FileInfo, error) {
		statCalls++
		return nil, os.ErrNotExist
	}

	require.NoError(t, runMain(context.Background(),
		[]string{"--dry-run", "client", "ip", "addr"}))

	assert.Equal(t, -1, s.ExitCode, "dry-run must not exit even with no marker on disk")
	assert.Equal(t, 0, statCalls, "dry-run must not consult Stat")
	testenv.AssertLines(t, s.Stdout.String(), []any{
		"install -d -m 0755 /run/npte/netns",
		`test -f /run/npte/netns/client || { echo 'npte: client: not managed by npte' >&2; exit 2; }`,
		"ip netns exec client runuser -u alice -- env ip addr",
	})
}
