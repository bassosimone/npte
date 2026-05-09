// SPDX-License-Identifier: GPL-3.0-or-later

package star

import (
	"context"
	"errors"
	"testing"

	"github.com/bassosimone/deferexit"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStarNetem(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantCmds [][]string
	}{{
		name:     "no profile only clears",
		args:     []string{"--dry-run"},
		wantExit: -1,
		wantCmds: [][]string{
			npte("netem", "clear", "router", "if-client", "-n"),
			npte("netem", "clear", "router", "if-server", "-n"),
		},
	}, {
		name:     "applies 4g-bloated profile",
		args:     []string{"--dry-run", "--profile", "4g-bloated"},
		wantExit: -1,
		wantCmds: [][]string{
			npte("netem", "clear", "router", "if-client", "-n"),
			npte("netem", "clear", "router", "if-server", "-n"),
			npte("netem", "apply", "--delay", "25ms", "--limit", "2500", "--rate", "30mbit", "router", "if-client", "-n"),
			npte("netem", "apply", "--delay", "25ms", "--limit", "1700", "--rate", "10mbit", "router", "if-server", "-n"),
		},
	}, {
		name:     "applies 4g-managed profile",
		args:     []string{"--dry-run", "--profile", "4g-managed"},
		wantExit: -1,
		wantCmds: [][]string{
			npte("netem", "clear", "router", "if-client", "-n"),
			npte("netem", "clear", "router", "if-server", "-n"),
			npte("netem", "apply", "--delay", "25ms", "--child", "cake", "--cake-bandwidth", "30mbit", "router", "if-client", "-n"),
			npte("netem", "apply", "--delay", "25ms", "--child", "cake", "--cake-bandwidth", "10mbit", "router", "if-server", "-n"),
		},
	}, {
		// Live-mode (no --dry-run): pass() returns argv unchanged, so
		// child invocations must not carry "-n".
		name:     "no profile only clears (live mode)",
		args:     nil,
		wantExit: -1,
		wantCmds: [][]string{
			npte("netem", "clear", "router", "if-client"),
			npte("netem", "clear", "router", "if-server"),
		},
	}, {
		name:     "rejects unknown profile",
		args:     []string{"--dry-run", "--profile", "bogus"},
		wantExit: 2,
		wantCmds: nil,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testenv.Setup(t)
			require.NoError(t, netemMain(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, s.ExitCode)
			assert.Equal(t, tc.wantCmds, s.Commands)
		})
	}
}

// TestApplyArgs_loss exercises the loss branch of applyArgs, which no
// curated profile currently triggers. The flag-emission order must match
// the canonical netem/apply.go ordering so dry-run output stays diffable.
func TestApplyArgs_loss(t *testing.T) {
	got := applyArgs("router", "if-client", linkShape{
		delay: "25ms",
		loss:  "1%",
		limit: "1000",
		rate:  "10mbit",
	})
	want := []string{
		"netem", "apply",
		"--delay", "25ms",
		"--loss", "1%",
		"--limit", "1000",
		"--rate", "10mbit",
		"router", "if-client",
	}
	assert.Equal(t, want, got)
}

// TestStarNetem_selfPathError pins the contract that selfPath logs and
// exits 1 when testable.Env.Executable fails. We swap Exit for
// deferexit.Panic and wrap the call in deferexit.Run so the exit halts
// netemMain (mirroring production), then assert no child npte ran.
func TestStarNetem_selfPathError(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.Exit = deferexit.Panic
	testable.Env.Executable = func() (string, error) {
		return "", errors.New("readlink: permission denied")
	}

	code := deferexit.Run(func() {
		require.NoError(t, netemMain(context.Background(), []string{"--dry-run"}))
	})

	assert.Equal(t, 1, code)
	assert.Empty(t, s.Commands, "no child npte must run after selfPath fails")
}
