// SPDX-License-Identifier: GPL-3.0-or-later

// Package clitest provides shared test helpers for CLI command packages.
//
// Each Setup call swaps testable.Env with stubs that:
//
//   - capture stdout, stderr and exit code in a returned [Stubs] value
//   - make registry.RequireManaged pass (Stat returns a fake regular file)
//   - make registry.Lock return a no-op unlock
//   - make RunCommand a no-op (no real subprocess is ever exec'd)
//
// Tests can therefore drive each leaf command with --dry-run and assert
// against the rendered shell script on stdout.
package clitest

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// SelfPath is the absolute path returned by the stubbed Executable; it is
// also the cmd.Path / cmd.Args[0] of every captured RunCommand entry that
// originates from `npte star`'s self-recursion.
const SelfPath = "/usr/local/sbin/npte"

// Stubs holds the buffers and the captured exit code for a test.
type Stubs struct {
	Stdout   *bytes.Buffer
	Stderr   *bytes.Buffer
	ExitCode int
	// SudoUser, if non-empty, is returned for $SUDO_USER lookups.
	SudoUser string
	// Commands records the full argv (including the binary as Args[0]) of
	// every command passed to the stubbed RunCommand. Useful for asserting
	// what `npte star` would dispatch, since runSelf always exec's even in
	// dry-run mode.
	Commands [][]string
}

// Setup swaps testable.Env with a stubbed [*testable.Environ] for the
// duration of t and returns the captures.
func Setup(t *testing.T) *Stubs {
	t.Helper()
	s := &Stubs{
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		ExitCode: -1,
	}
	orig := testable.Env
	t.Cleanup(func() { testable.Env = orig })
	testable.Env = &testable.Environ{
		Exit:        func(code int) { s.ExitCode = code },
		Stdin:       strings.NewReader(""),
		Stdout:      s.Stdout,
		Stderr:      s.Stderr,
		LogRenderer: lipgloss.NewRenderer(io.Discard),
		Getenv: func(key string) string {
			if key == "SUDO_USER" {
				return s.SudoUser
			}
			return ""
		},
		Geteuid:    func() int { return 0 },
		MkdirAll:   func(string, os.FileMode) error { return nil },
		ReadFile:   func(string) ([]byte, error) { return nil, nil },
		WriteFile:  func(string, []byte, os.FileMode) error { return nil },
		Stat:       func(name string) (os.FileInfo, error) { return regularFileInfo(name), nil },
		Remove:     func(string) error { return nil },
		ReadDir:    func(string) ([]os.DirEntry, error) { return nil, nil },
		RunCommand: func(cmd *exec.Cmd) error {
			argv := append([]string{}, cmd.Args...)
			s.Commands = append(s.Commands, argv)
			return nil
		},
		LookPath:   func(file string) (string, error) { return "/usr/bin/" + file, nil },
		Executable: func() (string, error) { return SelfPath, nil },
		LockFile:   func(string) (func(), error) { return func() {}, nil },
		LogFatalOnError0: func(err error) {
			if err != nil {
				t.Fatalf("unexpected internal error: %v", err)
			}
		},
	}
	return s
}

// regularFileInfo implements [os.FileInfo] for a fake regular file. It is
// returned by the stubbed Stat so that registry.RequireManaged accepts the
// path as a valid marker without touching the filesystem.
type regularFileInfo string

func (n regularFileInfo) Name() string     { return string(n) }
func (regularFileInfo) Size() int64        { return 0 }
func (regularFileInfo) Mode() fs.FileMode  { return 0o644 }
func (regularFileInfo) ModTime() time.Time { return time.Time{} }
func (regularFileInfo) IsDir() bool        { return false }
func (regularFileInfo) Sys() any           { return nil }

// AssertLines asserts that got, split by "\n" with a single trailing empty
// entry trimmed, equals want element-wise. Each want element is either a
// literal string (exact match) or a [*regexp.Regexp] (matched against the
// corresponding output line). Used to assert dry-run output shape.
func AssertLines(t *testing.T, got string, want []any) {
	t.Helper()
	lines := strings.Split(got, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	if !assert.Equal(t, len(want), len(lines), "stdout line count differs; got:\n%s", got) {
		return
	}
	for i, w := range want {
		switch v := w.(type) {
		case string:
			assert.Equal(t, v, lines[i], "line %d mismatch", i)
		case *regexp.Regexp:
			assert.Regexp(t, v, lines[i], "line %d does not match regex", i)
		default:
			t.Fatalf("AssertLines: unsupported want type %T at index %d", w, i)
		}
	}
}
