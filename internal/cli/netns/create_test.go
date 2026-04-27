// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"regexp"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	heredocOpen := regexp.MustCompile(`^install -D -m 0644 /dev/stdin /etc/netns/client/resolv\.conf <<'NPTE_EOF_[0-9A-F]+'$`)
	heredocClose := regexp.MustCompile(`^NPTE_EOF_[0-9A-F]+$`)

	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run happy path",
		args:     []string{"--dry-run", "client"},
		wantExit: -1,
		wantOut: []any{
			"install -d -m 0755 /run/npte/netns",
			"modprobe tcp_bbr",
			"ip netns add client",
			"ip netns exec client ip link set lo up",
			"ip netns exec client sysctl -w 'net.ipv4.tcp_rmem=4096 131072 33554432'",
			"ip netns exec client sysctl -w 'net.ipv4.tcp_wmem=4096 131072 33554432'",
			"ip netns exec client sysctl -w net.ipv4.ip_forward=1",
			heredocOpen,
			"nameserver 1.1.1.1",
			"nameserver 8.8.8.8",
			heredocClose,
			"install -m 0644 /dev/null /run/npte/netns/client",
		},
	}, {
		name:     "rejects bad name",
		args:     []string{"--dry-run", "1bad"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, createMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}
