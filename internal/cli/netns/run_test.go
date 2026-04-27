// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/cli/clitest"
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
			"ip netns exec client runuser -u alice -- env ip addr",
		},
	}, {
		name:     "dry-run with env vars",
		sudoUser: "alice",
		args:     []string{"--dry-run", "-e", "FOO=bar", "client", "ip", "addr"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
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
			s := clitest.Setup(t)
			s.SudoUser = tc.sudoUser
			require.NoError(t, runMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			clitest.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}
