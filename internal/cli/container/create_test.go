// SPDX-License-Identifier: GPL-3.0-or-later

package container

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
		args:     []string{"--dry-run", "noble", "/var/lib/machines/test"},
		wantExit: -1,
		wantOut: []any{
			"debootstrap noble /var/lib/machines/test",
		},
	}, {
		name:     "rejects invalid suite",
		args:     []string{"--dry-run", "1bad", "/var/lib/machines/test"},
		wantExit: 2,
	}, {
		name:     "rejects relative rootfs",
		args:     []string{"--dry-run", "noble", "rel/path"},
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
