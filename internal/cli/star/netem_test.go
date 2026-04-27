// SPDX-License-Identifier: GPL-3.0-or-later

package star

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/cli/clitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStarNetem(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantCmds [][]string
	}{{
		name:     "no profile only clears",
		args:     []string{"--dry-run"},
		wantExit: -1,
		wantCmds: [][]string{
			npte("netem", "clear", "router", "if-client", "-n"),
			npte("netem", "clear", "router", "if-server", "-n"),
		},
	}, {
		name:     "applies 4g-bloated profile",
		args:     []string{"--dry-run", "--profile", "4g-bloated"},
		wantExit: -1,
		wantCmds: [][]string{
			npte("netem", "clear", "router", "if-client", "-n"),
			npte("netem", "clear", "router", "if-server", "-n"),
			npte("netem", "apply", "--delay", "25ms", "--limit", "2500", "--rate", "30mbit", "router", "if-client", "-n"),
			npte("netem", "apply", "--delay", "25ms", "--limit", "1700", "--rate", "10mbit", "router", "if-server", "-n"),
		},
	}, {
		name:     "applies 4g-managed profile",
		args:     []string{"--dry-run", "--profile", "4g-managed"},
		wantExit: -1,
		wantCmds: [][]string{
			npte("netem", "clear", "router", "if-client", "-n"),
			npte("netem", "clear", "router", "if-server", "-n"),
			npte("netem", "apply", "--delay", "25ms", "--child", "cake", "--cake-bandwidth", "30mbit", "router", "if-client", "-n"),
			npte("netem", "apply", "--delay", "25ms", "--child", "cake", "--cake-bandwidth", "10mbit", "router", "if-server", "-n"),
		},
	}, {
		name:     "rejects unknown profile",
		args:     []string{"--dry-run", "--profile", "bogus"},
		wantExit: 2,
		wantCmds: nil,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := clitest.Setup(t)
			require.NoError(t, netemMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			assert.Equal(t, tc.wantCmds, s.Commands)
		})
	}
}
