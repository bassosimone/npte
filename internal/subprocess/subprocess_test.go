// SPDX-License-Identifier: GPL-3.0-or-later

package subprocess

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/stretchr/testify/assert"
)

// Make sure that MustRun actually runs a command. We re-invoke the test
// binary itself with a regex that matches no test, which exits 0 on every
// OS and keeps the test portable. We pretend that "ip" (an allowlisted
// name) resolves to os.Args[0] via a fake LookPath.
func TestMustRun(t *testing.T) {
	var captured error
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = io.Discard
	env.Stderr = io.Discard
	env.LookPath = func(string) (string, error) { return os.Args[0], nil }
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	MustRun(context.Background(), false, "ip", "-test.run=^$")
	assert.NoError(t, captured)
}

// Make sure that MustRun refuses to run commands that are not in the
// deps allowlist.
func TestMustRun_Disallowed(t *testing.T) {
	var captured error
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = io.Discard
	env.Stderr = io.Discard
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	MustRun(context.Background(), false, "nonexistent-bogus-cmd")
	assert.ErrorContains(t, captured, `command "nonexistent-bogus-cmd" is not in the allowlist`)
}

// Make sure that MustRun in dry mode prints the command to stdout
// rather than executing it. Dry-run does not enforce the allowlist.
func TestMustRun_Dry(t *testing.T) {
	var captured error
	var stdout bytes.Buffer
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = &stdout
	env.Stderr = io.Discard
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	MustRun(context.Background(), true, "echo", "hello world")
	assert.NoError(t, captured)
	assert.Equal(t, "echo 'hello world'\n", stdout.String())
}

// Make sure that MustPipeTo in dry mode prints a heredoc snippet that
// pastes cleanly into a shell.
func TestMustPipeTo_Dry(t *testing.T) {
	var captured error
	var stdout bytes.Buffer
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = &stdout
	env.Stderr = io.Discard
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	payload := []byte("nameserver 1.1.1.1\nnameserver 8.8.8.8\n")
	MustPipeTo(context.Background(), true, payload, "install",
		"-D", "-m", "0644", "/dev/stdin", "/etc/netns/foo/resolv.conf")
	assert.NoError(t, captured)

	out := stdout.String()
	assert.True(t, strings.HasPrefix(out,
		"install -D -m 0644 /dev/stdin /etc/netns/foo/resolv.conf <<'NPTE_EOF_"),
		"got: %s", out)
	assert.Contains(t, out, "\nnameserver 1.1.1.1\nnameserver 8.8.8.8\n")
	// Terminator must be on its own line (preceded and followed by "\n").
	assert.Regexp(t, `\nNPTE_EOF_[0-9A-F]+\n\z`, out)
}

// Make sure that MustPipeTo correctly terminates a heredoc even when the
// payload does not end with a newline.
func TestMustPipeTo_Dry_NoTrailingNewline(t *testing.T) {
	var captured error
	var stdout bytes.Buffer
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = &stdout
	env.Stderr = io.Discard
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	MustPipeTo(context.Background(), true, []byte("no newline"), "cat")
	assert.NoError(t, captured)
	assert.Regexp(t, `\nno newline\nNPTE_EOF_[0-9A-F]+\n\z`, stdout.String())
}

// Make sure that MustPipeTo actually pipes stdin to the command in live mode.
func TestMustPipeTo_Live(t *testing.T) {
	var captured error
	var stdout bytes.Buffer
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = &stdout
	env.Stderr = io.Discard
	env.LookPath = func(string) (string, error) { return "/bin/cat", nil }
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	MustPipeTo(context.Background(), false, []byte("piped body"), "install")
	assert.NoError(t, captured)
	assert.Equal(t, "piped body", stdout.String())
}
