// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/cli/clitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignAddr(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run happy path",
		args:     []string{"--dry-run", "client", "if-router", "10.99.0.2/30"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			"ip netns exec client ip addr add 10.99.0.2/30 dev if-router",
		},
	}, {
		name:     "rejects bad ns",
		args:     []string{"--dry-run", "1bad", "if-router", "10.99.0.2/30"},
		wantExit: 2,
	}, {
		name:     "rejects bad iface",
		args:     []string{"--dry-run", "client", "bad/iface", "10.99.0.2/30"},
		wantExit: 2,
	}, {
		name:     "rejects bad cidr",
		args:     []string{"--dry-run", "client", "if-router", "not-a-cidr"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := clitest.Setup(t)
			require.NoError(t, assignAddrMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			clitest.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}
