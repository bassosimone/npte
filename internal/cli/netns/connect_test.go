// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"testing"

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
