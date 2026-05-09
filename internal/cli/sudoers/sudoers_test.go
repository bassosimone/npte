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

// expectedHappyPath is the exact stdout the snippet must produce for
// user "alice". Pinning the whole output catches reordering, accidental
// added lines, and meaning-flips (e.g. "env_reset is on" vs "env_reset
// is off") that a substring-contains check would silently accept.
//
// If you intentionally change the snippet, update this literal in the
// same change so the test fails loudly with a clear diff for review.
const expectedHappyPath = `
# This snippet allows running the following commands without a password:
#
# - /usr/local/sbin/npte netns *
# - /usr/local/sbin/npte netem *
# - /usr/local/sbin/npte star *
#
# The user who is granted permission is the one who invoked
# the 'npte sudoers' command.

alice ALL=(root) NOPASSWD: /usr/local/sbin/npte netns *
alice ALL=(root) NOPASSWD: /usr/local/sbin/npte netem *
alice ALL=(root) NOPASSWD: /usr/local/sbin/npte star *

# Install this snippet by pasting it into /etc/sudoers via:
#
#     sudo visudo
#
# visudo validates the snippet's syntax before activating the change.
#
# Caveat: before applying these rules, make sure your sudoers config
# satisfies the following — npte's NOPASSWD surface relies on it,
# and overriding either silently turns this snippet into a
# passwordless privilege-escalation path:
#
# (a) env_reset is on, and env_keep does not add loader variables
#     (LD_PRELOAD, LD_LIBRARY_PATH, LD_AUDIT, ...) — npte does not
#     scrub the child environment in Go and trusts sudo to do it.
#
# (b) secure_path is set — npte and its privileged children resolve
#     helpers (tc, iptables modules, ...) by basename, so a caller-
#     controlled PATH redirects those lookups.
#
# These are sudo's defaults on every mainstream distro; the warning
# is here for the operator who has changed them.

`

func TestMain(t *testing.T) {
	tests := []struct {
		name          string
		euid          int
		user          string
		wantExit      int
		wantStdout    string
		wantStderrHas []string
	}{{
		name:       "happy path",
		euid:       1000,
		user:       "alice",
		wantExit:   -1,
		wantStdout: expectedHappyPath,
	}, {
		name:     "refuses when euid is 0",
		euid:     0,
		user:     "alice",
		wantExit: 2,
		wantStderrHas: []string{
			"adding a sudoers snippet for root does not make sense",
			"run this command as the user who should be allowlisted",
		},
	}, {
		name:          "refuses when USER is empty",
		euid:          1000,
		user:          "",
		wantExit:      2,
		wantStderrHas: []string{"$USER is not set"},
	}, {
		name:          "refuses when USER is malformed",
		euid:          1000,
		user:          "1bad",
		wantExit:      2,
		wantStderrHas: []string{"$USER", "must match"},
	}, {
		name:          "refuses when USER contains shell metacharacter",
		euid:          1000,
		user:          "alice;rm",
		wantExit:      2,
		wantStderrHas: []string{"$USER", "must match"},
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
			// On the happy path we pin the whole stdout; on refusal
			// paths stdout must be empty (no partial snippet leaks).
			assert.Equal(t, tc.wantStdout, stdout.String())
			for _, s := range tc.wantStderrHas {
				assert.Contains(t, stderr.String(), s)
			}
		})
	}
}
