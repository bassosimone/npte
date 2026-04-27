// SPDX-License-Identifier: GPL-3.0-or-later

package star

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/cli/clitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// npte builds the full argv of a child `npte` invocation, prefixed with the
// stubbed executable path so the assertion matches cmd.Args.
func npte(args ...string) []string {
	return append([]string{clitest.SelfPath}, args...)
}

func TestStarCreate(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantCmds [][]string
	}{{
		name:     "dry-run happy path",
		args:     []string{"--dry-run"},
		wantExit: -1,
		wantCmds: [][]string{
			npte("netns", "create", "client", "-n"),
			npte("netns", "create", "router", "-n"),
			npte("netns", "create", "server", "-n"),
			npte("netns", "connect", "client", "router", "-n"),
			npte("netns", "connect", "server", "router", "-n"),
			npte("netns", "assign-addr", "client", "if-router", "172.16.3.2/24", "-n"),
			npte("netns", "assign-addr", "router", "if-client", "172.16.3.1/24", "-n"),
			npte("netns", "assign-addr", "server", "if-router", "172.16.2.2/24", "-n"),
			npte("netns", "assign-addr", "router", "if-server", "172.16.2.1/24", "-n"),
			npte("netns", "add-route", "client", "default", "172.16.3.1", "-n"),
			npte("netns", "add-route", "server", "default", "172.16.2.1", "-n"),
		},
	}, {
		name:     "live mode omits -n",
		args:     nil,
		wantExit: -1,
		wantCmds: [][]string{
			npte("netns", "create", "client"),
			npte("netns", "create", "router"),
			npte("netns", "create", "server"),
			npte("netns", "connect", "client", "router"),
			npte("netns", "connect", "server", "router"),
			npte("netns", "assign-addr", "client", "if-router", "172.16.3.2/24"),
			npte("netns", "assign-addr", "router", "if-client", "172.16.3.1/24"),
			npte("netns", "assign-addr", "server", "if-router", "172.16.2.2/24"),
			npte("netns", "assign-addr", "router", "if-server", "172.16.2.1/24"),
			npte("netns", "add-route", "client", "default", "172.16.3.1"),
			npte("netns", "add-route", "server", "default", "172.16.2.1"),
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := clitest.Setup(t)
			require.NoError(t, createMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			assert.Equal(t, tc.wantCmds, s.Commands)
		})
	}
}
