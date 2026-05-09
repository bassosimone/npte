// SPDX-License-Identifier: GPL-3.0-or-later

package testenv

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_capturesExitAndStreams(t *testing.T) {
	s := Setup(t)
	testable.Env.Stdout.Write([]byte("out\n"))
	testable.Env.Stderr.Write([]byte("err\n"))
	testable.Env.Exit(7)

	assert.Equal(t, "out\n", s.Stdout.String())
	assert.Equal(t, "err\n", s.Stderr.String())
	assert.Equal(t, 7, s.ExitCode)
}

func TestSetup_restoresEnvOnCleanup(t *testing.T) {
	orig := testable.Env
	t.Run("inner", func(t *testing.T) {
		Setup(t)
		assert.NotSame(t, orig, testable.Env)
	})
	assert.Same(t, orig, testable.Env)
}

func TestSetup_sudoUser(t *testing.T) {
	s := Setup(t)
	s.SudoUser = "alice"
	assert.Equal(t, "alice", testable.Env.Getenv("SUDO_USER"))
	assert.Equal(t, "", testable.Env.Getenv("OTHER"))
}

func TestSetup_lstatReturnsRegularFile(t *testing.T) {
	Setup(t)
	info, err := testable.Env.Lstat("/some/path")
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
	assert.False(t, info.IsDir())
	assert.Equal(t, "/some/path", info.Name())
	assert.Zero(t, info.Size())
	assert.True(t, info.ModTime().IsZero())
	assert.Nil(t, info.Sys())
}

func TestSetup_otherStubsAreNoOps(t *testing.T) {
	Setup(t)
	assert.NoError(t, testable.Env.MkdirAll("/x", 0o755))
	assert.NoError(t, testable.Env.WriteFile("/x", nil, 0o644))
	assert.NoError(t, testable.Env.Remove("/x"))
	data, err := testable.Env.ReadFile("/x")
	assert.NoError(t, err)
	assert.Nil(t, data)
	entries, err := testable.Env.ReadDir("/x")
	assert.NoError(t, err)
	assert.Empty(t, entries)
	path, err := testable.Env.LookPath("ip")
	assert.NoError(t, err)
	assert.Equal(t, "/usr/bin/ip", path)
	assert.Equal(t, 0, testable.Env.Geteuid())
	unlock, err := testable.Env.LockFile("/x")
	assert.NoError(t, err)
	unlock()

	// Getwd is consulted by netns/run.go for the sandbox workdir.
	wd, err := testable.Env.Getwd()
	assert.NoError(t, err)
	assert.Equal(t, "/home/sbs/src/github.com/bassosimone/npte", wd)

	// Executable is consulted by star/* for the npte self-recursion path.
	self, err := testable.Env.Executable()
	assert.NoError(t, err)
	assert.Equal(t, SelfPath, self)
}

// TestSetup_runCommandRecordsArgv pins the contract that the stubbed
// RunCommand captures cmd.Args (not just cmd.Path) into Stubs.Commands
// and returns nil, so live-mode tests can assert what would have been
// exec'd without needing --dry-run.
func TestSetup_runCommandRecordsArgv(t *testing.T) {
	s := Setup(t)
	cmd := exec.Command("/usr/bin/ip", "netns", "list")

	require.NoError(t, testable.Env.RunCommand(cmd))
	require.NoError(t, testable.Env.RunCommand(exec.Command("/usr/bin/tc", "qdisc", "show")))

	assert.Equal(t, [][]string{
		{"/usr/bin/ip", "netns", "list"},
		{"/usr/bin/tc", "qdisc", "show"},
	}, s.Commands)
}

// TestSetup_logFatalOnError0NilIsNoOp covers the nil-error fast path of
// the LogFatalOnError0 stub. The non-nil branch calls t.Fatalf which
// would fail the calling test by design, so it is deliberately not
// exercised here — see [Setup] for why that asymmetry is intentional.
func TestSetup_logFatalOnError0NilIsNoOp(t *testing.T) {
	Setup(t)
	assert.NotPanics(t, func() {
		testable.Env.LogFatalOnError0(nil)
	})
}

func TestAssertLines_literalMatch(t *testing.T) {
	AssertLines(t, "a\nb\n", []any{"a", "b"})
}

func TestAssertLines_regexMatch(t *testing.T) {
	AssertLines(t, "hello-1234\n", []any{regexp.MustCompile(`^hello-\d+$`)})
}

func TestAssertLines_mixedMatch(t *testing.T) {
	AssertLines(t, "literal\nregex-99\n",
		[]any{"literal", regexp.MustCompile(`^regex-\d+$`)})
}

func TestAssertLines_noTrailingNewline(t *testing.T) {
	AssertLines(t, "a\nb", []any{"a", "b"})
}

func TestAssertLines_emptyMatch(t *testing.T) {
	AssertLines(t, "", nil)
}
