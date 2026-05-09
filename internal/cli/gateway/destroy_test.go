// SPDX-License-Identifier: GPL-3.0-or-later

package gateway

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
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
		args:     []string{"--dry-run", "router"},
		wantExit: -1,
		wantOut: []any{
			`iptables-save | grep -Fv -- '--comment "npte:gw:router"' | iptables-restore`,
			"ip link del if-router || true",
		},
	}, {
		name:     "rejects bad ns",
		args:     []string{"--dry-run", "1bad"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, destroyMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}
