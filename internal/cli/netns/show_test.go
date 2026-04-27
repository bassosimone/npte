// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShow(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantOut  []any
	}{{
		// show does not support --dry-run; the stubbed RunCommand returns
		// nil, so all section commands "succeed" with no output. We assert
		// just the section headers and the trailing blank lines emitted by
		// show itself.
		name:     "all sections",
		args:     []string{"client"},
		wantExit: -1,
		wantOut: []any{
			"=== link ===", "",
			"=== addr ===", "",
			"=== route ===", "",
			"=== route6 ===", "",
			"=== qdisc ===", "",
			"=== neigh ===", "",
			"=== sockets ===", "",
		},
	}, {
		name:     "filtered sections",
		args:     []string{"--section", "route", "--section", "qdisc", "client"},
		wantExit: -1,
		wantOut: []any{
			"=== route ===", "",
			"=== qdisc ===", "",
		},
	}, {
		name:     "rejects bad ns",
		args:     []string{"1bad"},
		wantExit: 2,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, showMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			testenv.AssertLines(t, s.Stdout.String(), tc.wantOut)
		})
	}
}
