// SPDX-License-Identifier: GPL-3.0-or-later

package testenv

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTB satisfies [TB] without calling out to a real *testing.T, so
// tests for the testenv helpers themselves can exercise the t.Errorf /
// t.Fatalf branches without failing the parent test. Calls are recorded
// verbatim; we deliberately do not abort on Fatalf — the only caller
// that hits Fatalf in production code (AssertLines's default case) sits
// at the tail of a switch inside a loop, and the surrounding test feeds
// only one bad element.
type fakeTB struct {
	helperCalls int
	cleanups    []func()
	errorfMsgs  []string
	fatalfMsgs  []string
}

func (f *fakeTB) Helper()           { f.helperCalls++ }
func (f *fakeTB) Cleanup(fn func()) { f.cleanups = append(f.cleanups, fn) }
func (f *fakeTB) Errorf(s string, a ...any) {
	f.errorfMsgs = append(f.errorfMsgs, fmt.Sprintf(s, a...))
}
func (f *fakeTB) Fatalf(s string, a ...any) {
	f.fatalfMsgs = append(f.fatalfMsgs, fmt.Sprintf(s, a...))
}

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

	// Executable is consulted by lab/* for the npte self-recursion path.
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

func TestSetup_startCommandRecordsArgv(t *testing.T) {
	s := Setup(t)
	cmd := exec.Command("/usr/bin/sudo", "-n", "/usr/local/bin/npte", "netns", "create", "foo")

	require.NoError(t, testable.Env.StartCommand(cmd))

	assert.Equal(t, [][]string{
		{"/usr/bin/sudo", "-n", "/usr/local/bin/npte", "netns", "create", "foo"},
	}, s.Commands)
}

func TestSetup_waitCommandReturnsNil(t *testing.T) {
	Setup(t)
	cmd := exec.Command("/bin/true")
	assert.NoError(t, testable.Env.WaitCommand(cmd))
}

func TestSetup_processSignalReturnsNil(t *testing.T) {
	Setup(t)
	cmd := exec.Command("/bin/true")
	assert.NoError(t, testable.Env.ProcessSignal(cmd, nil))
}

// TestSetup_logFatalOnError0NilIsNoOp covers the nil-error fast path of
// the LogFatalOnError0 stub.
func TestSetup_logFatalOnError0NilIsNoOp(t *testing.T) {
	Setup(t)
	assert.NotPanics(t, func() {
		testable.Env.LogFatalOnError0(nil)
	})
}

// TestSetup_logFatalOnError0NonNilCallsFatalf covers the failure branch
// of the LogFatalOnError0 stub by passing in a fakeTB and observing
// that Fatalf is invoked with the wrapped error.
func TestSetup_logFatalOnError0NonNilCallsFatalf(t *testing.T) {
	orig := testable.Env
	t.Cleanup(func() { testable.Env = orig })

	fake := &fakeTB{}
	Setup(fake)
	testable.Env.LogFatalOnError0(errors.New("boom"))

	require.Len(t, fake.fatalfMsgs, 1)
	assert.Contains(t, fake.fatalfMsgs[0], "unexpected internal error")
	assert.Contains(t, fake.fatalfMsgs[0], "boom")
	assert.Empty(t, fake.errorfMsgs)
	// Setup registers a Cleanup to restore testable.Env; the fake
	// captures it but does not auto-run, so we restore manually above.
	assert.Len(t, fake.cleanups, 1)
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

// TestAssertLines_lineCountMismatch covers the early-return branch when
// the number of want elements differs from the number of output lines.
// Using a real *testing.T would propagate the failure to the parent, so
// we drive AssertLines via a fakeTB and observe that exactly one Errorf
// is recorded (from assert.Equal on the lengths) and Fatalf is not.
func TestAssertLines_lineCountMismatch(t *testing.T) {
	fake := &fakeTB{}
	AssertLines(fake, "only-one-line\n", []any{"a", "b"})

	require.Len(t, fake.errorfMsgs, 1)
	assert.Contains(t, fake.errorfMsgs[0], "stdout line count differs")
	assert.Empty(t, fake.fatalfMsgs)
}

// TestAssertLines_unsupportedWantType covers the default switch branch
// that rejects a want element whose type is neither string nor
// *regexp.Regexp. We feed exactly one bad element so that the loop
// body is hit once and the recorded Fatalf naming is unambiguous.
func TestAssertLines_unsupportedWantType(t *testing.T) {
	fake := &fakeTB{}
	AssertLines(fake, "anything\n", []any{42})

	require.Len(t, fake.fatalfMsgs, 1)
	assert.Contains(t, fake.fatalfMsgs[0], "unsupported want type")
	assert.Contains(t, fake.fatalfMsgs[0], "int")
	assert.Empty(t, fake.errorfMsgs)
}
