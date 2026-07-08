// SPDX-License-Identifier: GPL-3.0-or-later

package gencerts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mkdirAll      func(string, os.FileMode) error
		writeFile     func(string, []byte, os.FileMode) error
		wantExit      int
		wantStderrHas []string
		wantFiles     []string
	}{{
		name:      "defaults: 127.0.0.1 as IP and DNS SAN",
		wantExit:  -1,
		wantFiles: []string{"cert.pem", "key.pem"},
	}, {
		name:      "custom IP only: DNS defaults to IP",
		args:      []string{"--ip-addr", "10.0.0.1"},
		wantExit:  -1,
		wantFiles: []string{"cert.pem", "key.pem"},
	}, {
		name:      "custom DNS only: IP defaults to 127.0.0.1",
		args:      []string{"--dns-name", "foo.example.com"},
		wantExit:  -1,
		wantFiles: []string{"cert.pem", "key.pem"},
	}, {
		name:      "custom output directory",
		args:      []string{"-C", "mydir"},
		wantExit:  -1,
		wantFiles: []string{"mydir/cert.pem", "mydir/key.pem"},
	}, {
		name:          "invalid IP address",
		args:          []string{"--ip-addr", "notanip"},
		wantExit:      2,
		wantStderrHas: []string{"invalid IP address", "notanip"},
	}, {
		name: "MkdirAll failure",
		args: []string{"-C", "baddir"},
		mkdirAll: func(string, os.FileMode) error {
			return errors.New("mocked MkdirAll failure")
		},
		wantExit: 1,
	}, {
		name: "WriteFile cert.pem failure",
		writeFile: func(name string, _ []byte, _ os.FileMode) error {
			if strings.HasSuffix(name, "cert.pem") {
				return errors.New("mocked WriteFile failure")
			}
			return nil
		},
		wantExit: 1,
	}, {
		name: "WriteFile key.pem failure",
		writeFile: func(name string, _ []byte, _ os.FileMode) error {
			if strings.HasSuffix(name, "key.pem") {
				return errors.New("mocked WriteFile failure")
			}
			return nil
		},
		wantExit: 1,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			exitCode := -1
			writtenFiles := make(map[string][]byte)

			mkdirAll := tc.mkdirAll
			if mkdirAll == nil {
				mkdirAll = func(string, os.FileMode) error { return nil }
			}
			writeFile := tc.writeFile
			if writeFile == nil {
				writeFile = func(name string, data []byte, _ os.FileMode) error {
					writtenFiles[name] = data
					return nil
				}
			}

			orig := testable.Env
			t.Cleanup(func() { testable.Env = orig })
			testable.Env = &testable.Environ{
				Exit:        func(code int) { exitCode = code },
				Stdout:      io.Discard,
				Stderr:      &stderr,
				LogRenderer: lipgloss.NewRenderer(io.Discard),
				MkdirAll:    mkdirAll,
				WriteFile:   writeFile,
				LogFatalOnError0: func(err error) {
					if err != nil {
						exitCode = 1
					}
				},
			}

			require.NoError(t, Main(context.Background(), tc.args))
			assert.Equal(t, tc.wantExit, exitCode)
			for _, s := range tc.wantStderrHas {
				assert.Contains(t, stderr.String(), s)
			}
			for _, f := range tc.wantFiles {
				assert.NotEmpty(t, writtenFiles[f], "expected %s to be written", f)
			}
		})
	}
}
