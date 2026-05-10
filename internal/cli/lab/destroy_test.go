// SPDX-License-Identifier: GPL-3.0-or-later

package lab

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabDestroy(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantCmds [][]string
	}{{
		name:     "dry-run",
		args:     []string{"--dry-run"},
		wantExit: -1,
		wantCmds: [][]string{
			npte("netns", "destroy", "client", "-n"),
			npte("netns", "destroy", "server", "-n"),
			npte("netns", "destroy", "router", "-n"),
		},
	}, {
		name:     "live",
		args:     nil,
		wantExit: -1,
		wantCmds: [][]string{
			npte("netns", "destroy", "client"),
			npte("netns", "destroy", "server"),
			npte("netns", "destroy", "router"),
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, destroyMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			assert.Equal(t, tc.wantCmds, s.Commands)
		})
	}
}
