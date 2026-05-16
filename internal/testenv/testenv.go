// SPDX-License-Identifier: GPL-3.0-or-later

// Package testenv provides a shared test stub for [testable.Env].
//
// Each [Setup] call swaps [testable.Env] with stubs that:
//
//   - capture stdout, stderr and exit code in the returned [Stubs] value
//   - make registry.RequireManaged pass (Stat returns a fake regular file)
//   - make registry.Lock return a no-op unlock
//   - capture every RunCommand argv into Stubs.Commands (no real exec)
//
// Tests can drive each leaf CLI command with --dry-run and assert against
// the rendered shell script on stdout, or exercise lower layers directly
// while overriding individual stubs.
package testenv

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/bassosimone/npte/internal/buildcfg"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// TB is the subset of [testing.TB] used by [Setup] and [AssertLines].
// We accept this narrower interface (rather than *testing.T directly) so
// that unit tests for the testenv helpers themselves can pass in a stub
// that records Errorf / Fatalf without failing the parent test —
// *testing.T can't be faked from outside the testing package because of
// its unexported private() marker, but the surface we actually need is
// small and easy to satisfy.
type TB interface {
	Cleanup(func())
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// SelfPath is the absolute path returned by the stubbed Executable; it is
// also the cmd.Path / cmd.Args[0] of every captured RunCommand entry that
// originates from `npte lab`'s self-recursion. It delegates to
// [buildcfg.InstallPath] so the test stubs and the sudoers snippet always
// agree on the canonical path.
var SelfPath = buildcfg.InstallPath

// Stubs holds the buffers and the captured exit code for a test.
type Stubs struct {
	Stdout   *bytes.Buffer
	Stderr   *bytes.Buffer
	ExitCode int
	// SudoUser, if non-empty, is returned for $SUDO_USER lookups.
	SudoUser string
	// Commands records the full argv (including the binary as Args[0]) of
	// every command passed to the stubbed RunCommand. Useful for asserting
	// what `npte lab` would dispatch, since runSelf always exec's even in
	// dry-run mode.
	Commands [][]string
}

// Setup swaps testable.Env with a stubbed [*testable.Environ] for the
// duration of t and returns the captures.
func Setup(t TB) *Stubs {
	t.Helper()
	s := &Stubs{
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		ExitCode: -1,
	}
	orig := testable.Env
	t.Cleanup(func() { testable.Env = orig })
	testable.Env = &testable.Environ{
		Args:        os.Args,
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
		Geteuid:   func() int { return 0 },
		Getwd:     func() (string, error) { return "/home/sbs/src/github.com/bassosimone/npte", nil },
		MkdirAll:  func(string, os.FileMode) error { return nil },
		ReadFile:  func(string) ([]byte, error) { return nil, nil },
		WriteFile: func(string, []byte, os.FileMode) error { return nil },
		Lstat:     func(name string) (os.FileInfo, error) { return regularFileInfo(name), nil },
		Remove:    func(string) error { return nil },
		ReadDir:   func(string) ([]os.DirEntry, error) { return nil, nil },
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
// returned by the stubbed Lstat so that registry.RequireManaged accepts
// the path as a valid marker without touching the filesystem.
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
func AssertLines(t TB, got string, want []any) {
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
