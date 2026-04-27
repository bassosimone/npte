// SPDX-License-Identifier: GPL-3.0-or-later

package container

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoot(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		name:     "dry-run no netns",
		args:     []string{"--dry-run", "/var/lib/machines/test"},
		wantExit: -1,
		wantOut: []any{
			"systemd-nspawn --boot -D /var/lib/machines/test",
		},
	}, {
		name:     "dry-run with netns and bind",
		args:     []string{"--dry-run", "--netns", "client", "--bind", "/dev/net/tun", "/var/lib/machines/test"},
		wantExit: -1,
		wantOut: []any{
			"systemd-nspawn --boot -D /var/lib/machines/test --network-namespace-path=/run/netns/client --bind=/dev/net/tun",
		},
	}, {
		name:     "rejects relative rootfs",
		args:     []string{"--dry-run", "rel/path"},
		wantExit: 2,
	}, {
		name:     "rejects bad netns name",
		args:     []string{"--dry-run", "--netns", "1bad", "/var/lib/machines/test"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, bootMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}
