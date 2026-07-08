// SPDX-License-Identifier: GPL-3.0-or-later

package gateway

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run happy path",
		args:     []string{"--dry-run", "router", "10.99.0.0/30", "eth0"},
		wantExit: -1,
		wantOut: []any{
			"ip link add if-router type veth peer name if-host",
			"ip link set if-host netns router",
			"ip addr add 10.99.0.1/30 dev if-router",
			"ip netns exec router ip addr add 10.99.0.2/30 dev if-host",
			"ip link set if-router up",
			"ip netns exec router ip link set if-host up",
			"ip netns exec router ip route add default via 10.99.0.1",
			"sysctl -w net.ipv4.ip_forward=1",
			"iptables -t nat -A POSTROUTING -s 10.99.0.0/30 -o eth0 -m comment --comment npte:gw:router -j MASQUERADE",
			"iptables -I FORWARD -s 10.99.0.0/30 -m comment --comment npte:gw:router -j ACCEPT",
			"iptables -I FORWARD -d 10.99.0.0/30 -m comment --comment npte:gw:router -j ACCEPT",
			"ip netns exec router iptables -t nat -A POSTROUTING -o if-host -m comment --comment npte:gw:router -j SNAT --to-source 10.99.0.2",
		},
	}, {
		// Host bits in <subnet> are ignored: host/ns addresses derive
		// from the masked prefix, and the iptables rules must use the
		// canonical network address, not the raw argument.
		name:     "dry-run canonicalizes subnet host bits",
		args:     []string{"--dry-run", "router", "10.99.0.1/30", "eth0"},
		wantExit: -1,
		wantOut: []any{
			"ip link add if-router type veth peer name if-host",
			"ip link set if-host netns router",
			"ip addr add 10.99.0.1/30 dev if-router",
			"ip netns exec router ip addr add 10.99.0.2/30 dev if-host",
			"ip link set if-router up",
			"ip netns exec router ip link set if-host up",
			"ip netns exec router ip route add default via 10.99.0.1",
			"sysctl -w net.ipv4.ip_forward=1",
			"iptables -t nat -A POSTROUTING -s 10.99.0.0/30 -o eth0 -m comment --comment npte:gw:router -j MASQUERADE",
			"iptables -I FORWARD -s 10.99.0.0/30 -m comment --comment npte:gw:router -j ACCEPT",
			"iptables -I FORWARD -d 10.99.0.0/30 -m comment --comment npte:gw:router -j ACCEPT",
			"ip netns exec router iptables -t nat -A POSTROUTING -o if-host -m comment --comment npte:gw:router -j SNAT --to-source 10.99.0.2",
		},
	}, {
		name:     "rejects bad ns",
		args:     []string{"--dry-run", "1bad", "10.99.0.0/30", "eth0"},
		wantExit: 2,
	}, {
		name:     "rejects bad subnet",
		args:     []string{"--dry-run", "router", "not-a-cidr", "eth0"},
		wantExit: 2,
	}, {
		name:     "rejects subnet too small",
		args:     []string{"--dry-run", "router", "10.99.0.0/31", "eth0"},
		wantExit: 2,
	}, {
		name:     "rejects ipv6 subnet",
		args:     []string{"--dry-run", "router", "fc00::/64", "eth0"},
		wantExit: 2,
	}, {
		// extIface flows through validate.IfaceName before any kernel
		// command is emitted; a value with shell metacharacters must be
		// rejected with exit 2.
		name:     "rejects bad ext-iface",
		args:     []string{"--dry-run", "router", "10.99.0.0/30", "bad iface"},
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
