// SPDX-License-Identifier: GPL-3.0-or-later

package sudoers

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	tests := []struct {
		name             string
		euid             int
		user             string
		wantExit         int
		wantStdoutHas    []string
		wantStderrHas    []string
		wantStdoutNotHas []string
	}{{
		name:     "happy path",
		euid:     1000,
		user:     "alice",
		wantExit: -1,
		wantStdoutHas: []string{
			"# This snippet allows running the following commands without a password:",
			"# - /usr/local/sbin/npte netns *",
			"# - /usr/local/sbin/npte netem *",
			"alice ALL=(root) NOPASSWD: /usr/local/sbin/npte netns *",
			"alice ALL=(root) NOPASSWD: /usr/local/sbin/npte netem *",
			"visudo",
		},
		wantStdoutNotHas: []string{
			"Cmnd_Alias",
			"Defaults!",
			"env_reset",
			"secure_path",
			"sudo install",
			"<<'EOF'",
		},
	}, {
		name:     "refuses when euid is 0",
		euid:     0,
		user:     "alice",
		wantExit: 2,
		wantStderrHas: []string{
			"adding a sudoers snippet for root does not make sense",
			"run this command as the user who should be allowlisted",
		},
		wantStdoutNotHas: []string{"NOPASSWD"},
	}, {
		name:             "refuses when USER is empty",
		euid:             1000,
		user:             "",
		wantExit:         2,
		wantStderrHas:    []string{"$USER is not set"},
		wantStdoutNotHas: []string{"NOPASSWD"},
	}, {
		name:             "refuses when USER is malformed",
		euid:             1000,
		user:             "1bad",
		wantExit:         2,
		wantStderrHas:    []string{"$USER", "must match"},
		wantStdoutNotHas: []string{"NOPASSWD"},
	}, {
		name:             "refuses when USER contains shell metacharacter",
		euid:             1000,
		user:             "alice;rm",
		wantExit:         2,
		wantStderrHas:    []string{"$USER", "must match"},
		wantStdoutNotHas: []string{"NOPASSWD"},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := -1

			orig := testable.Env
			t.Cleanup(func() { testable.Env = orig })
			testable.Env = &testable.Environ{
				Exit:        func(code int) { exitCode = code },
				Stdout:      &stdout,
				Stderr:      &stderr,
				LogRenderer: lipgloss.NewRenderer(io.Discard),
				Geteuid:     func() int { return tc.euid },
				Getenv: func(key string) string {
					if key == "USER" {
						return tc.user
					}
					return ""
				},
			}

			require.NoError(t, Main(context.Background(), nil))
			assert.Equal(t, tc.wantExit, exitCode)
			for _, s := range tc.wantStdoutHas {
				assert.Contains(t, stdout.String(), s)
			}
			for _, s := range tc.wantStderrHas {
				assert.Contains(t, stderr.String(), s)
			}
			for _, s := range tc.wantStdoutNotHas {
				assert.NotContains(t, stdout.String(), s)
			}
		})
	}
}
