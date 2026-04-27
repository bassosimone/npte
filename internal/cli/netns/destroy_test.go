// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/cli/clitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestroy(t *testing.T) {
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
			"rm -rf /etc/netns/client",
			"ip netns del client",
			"rm -f /run/npte/netns/client",
		},
	}, {
		name:     "rejects bad name",
		args:     []string{"--dry-run", "1bad"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := clitest.Setup(t)
			require.NoError(t, destroyMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			clitest.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}
