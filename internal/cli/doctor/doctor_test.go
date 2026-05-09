// SPDX-License-Identifier: GPL-3.0-or-later

package doctor

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain_Dependencies(t *testing.T) {
	allPresent := map[string]string{
		"ip":               "/usr/sbin/ip",
		"tc":               "/usr/sbin/tc",
		"iptables":         "/usr/sbin/iptables",
		"iptables-save":    "/usr/sbin/iptables-save",
		"iptables-restore": "/usr/sbin/iptables-restore",
		"grep":             "/usr/bin/grep",
		"sysctl":           "/usr/sbin/sysctl",
		"modprobe":         "/usr/sbin/modprobe",
		"install":          "/usr/bin/install",
		"env":              "/usr/bin/env",
		"rm":               "/usr/bin/rm",
		"runuser":          "/usr/sbin/runuser",
		"systemd-nspawn":   "/usr/bin/systemd-nspawn",
		"debootstrap":      "/usr/sbin/debootstrap",
		"bwrap":            "/usr/bin/bwrap",
	}

	tests := []struct {
		name         string
		found        map[string]string
		wantExit     int
		wantContains []string
		wantMissing  []string // substrings that must NOT appear
	}{{
		name:        "all present",
		found:       allPresent,
		wantExit:    -1,
		wantMissing: []string{"MISSING", "apt install"},
	}, {
		// Both `ip` and `tc` ship in the iproute2 Debian package, so when
		// both are absent the apt-install hint must list iproute2 only once.
		name: "ip and tc missing: iproute2 listed only once",
		found: map[string]string{
			"iptables":         "/usr/sbin/iptables",
			"iptables-save":    "/usr/sbin/iptables-save",
			"iptables-restore": "/usr/sbin/iptables-restore",
			"grep":             "/usr/bin/grep",
			"sysctl":           "/usr/sbin/sysctl",
			"modprobe":         "/usr/sbin/modprobe",
			"install":          "/usr/bin/install",
			"env":              "/usr/bin/env",
			"rm":               "/usr/bin/rm",
			"runuser":          "/usr/sbin/runuser",
			"systemd-nspawn":   "/usr/bin/systemd-nspawn",
			"debootstrap":      "/usr/sbin/debootstrap",
			"bwrap":            "/usr/bin/bwrap",
		},
		wantExit:     1,
		wantContains: []string{"MISSING (iproute2)", "apt install iproute2\n"},
	}, {
		name:     "all missing",
		found:    nil,
		wantExit: 1,
		wantContains: []string{
			"MISSING",
			"apt install iproute2 iptables grep procps kmod coreutils util-linux systemd-container debootstrap bubblewrap",
		},
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
				LookPath: func(file string) (string, error) {
					if path, ok := tc.found[file]; ok {
						return path, nil
					}
					return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
				},
			}

			require.NoError(t, Main(context.Background(), nil))
			assert.Equal(t, tc.wantExit, exitCode)
			for _, s := range tc.wantContains {
				assert.Contains(t, stdout.String(), s)
			}
			for _, s := range tc.wantMissing {
				assert.NotContains(t, stdout.String(), s)
			}
		})
	}
}
