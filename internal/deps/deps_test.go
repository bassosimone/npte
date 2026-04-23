// SPDX-License-Identifier: GPL-3.0-or-later

package deps

import (
	"errors"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/stretchr/testify/assert"
)

func TestLookPath(t *testing.T) {
	sentinel := errors.New("not found")

	tests := []struct {
		name     string
		input    string
		fakePath string
		fakeErr  error
		wantPath string
		wantErr  string // empty means no error, otherwise a substring match
	}{{
		name:    "disallowed command",
		input:   "curl",
		wantErr: `deps: command "curl" is not in the allowlist`,
	}, {
		name:     "allowed and found",
		input:    "ip",
		fakePath: "/usr/sbin/ip",
		wantPath: "/usr/sbin/ip",
	}, {
		name:    "allowed but missing",
		input:   "ip",
		fakeErr: sentinel,
		wantErr: "not found",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orig := testable.Env
			t.Cleanup(func() { testable.Env = orig })
			testable.Env = &testable.Environ{
				LookPath: func(string) (string, error) {
					return tc.fakePath, tc.fakeErr
				},
			}

			path, err := LookPath(tc.input)
			assert.Equal(t, tc.wantPath, path)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
