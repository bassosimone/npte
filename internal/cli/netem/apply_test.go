// SPDX-License-Identifier: GPL-3.0-or-later

package netem

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/cli/clitest"
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
			"ip netns exec client tc qdisc add dev if-router root handle 1: netem delay 10ms",
		},
	}, {
		name:     "dry-run with cake child",
		args:     []string{"--dry-run", "--delay", "10ms", "--child", "cake", "--cake-bandwidth", "30mbit", "client", "if-router"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
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
		// Child-kind validation runs after the root netem command is emitted,
		// so the lock prefix and the root line still appear on stdout.
		name:     "rejects bad child kind",
		args:     []string{"--dry-run", "--delay", "10ms", "--child", "bogus", "client", "if-router"},
		wantExit: 2,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			"ip netns exec client tc qdisc add dev if-router root handle 1: netem delay 10ms",
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := clitest.Setup(t)
			require.NoError(t, applyMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			clitest.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}
