// SPDX-License-Identifier: GPL-3.0-or-later

// Package subprocess contains code to exec subprocesses.
//
// TODO(bassosimone): dry-run output is not round-trippable when an argv
// word contains `#`: go-shellquote does not quote it, so the rendered
// line re-parses with everything after `#` as a comment (e.g., `tc qdisc
// del dev if#0 root || true` targets `if` and loses the guard). This is
// reachable because validate.IfaceName permits `#` and `netns run`
// forwards arbitrary command bytes. Affects every render site here and
// in pipeline.go. Fixing means force-quoting words containing `#` (fork
// go-shellquote or own the quoter) and/or rejecting `#` in IfaceName,
// plus an adversarial render→shellquote.Split round-trip test.
package subprocess

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bassosimone/npte/internal/deps"
	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/kballard/go-shellquote"
)

// Run runs the command represented by the given argv.
//
// The argv0 must be a bare command name in [deps.All]; Run resolves it to
// an absolute path via [deps.LookPath] before execution.
//
// When the dryRun argument is true the command is not executed but rather
// the command that would be executed is printed on stdout. Dry-run does
// not resolve or enforce the allowlist because it has no side effects.
func Run(ctx context.Context, dryRun bool, argv0 string, args ...string) error {
	env := testable.Env
	argv := append([]string{argv0}, args...)
	quoted := shellquote.Join(argv...)

	if dryRun {
		_, err := fmt.Fprintf(env.Stdout, "%s\n", quoted)
		return err
	}

	path, err := deps.LookPath(argv0)
	if err != nil {
		return err
	}

	logx.Command("%s", quoted)

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = env.Stdin
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr

	return env.RunCommand(cmd)
}

// MustRun is like [Run] but logs and exits on failure.
func MustRun(ctx context.Context, dryRun bool, argv0 string, args ...string) {
	testable.Env.LogFatalOnError0(Run(ctx, dryRun, argv0, args...))
}

// PipeTo runs the command represented by argv, piping stdin to its standard
// input. The argv0 resolves through the [deps.All] allowlist just like [Run].
//
// When dryRun is true, PipeTo prints a round-trippable shell snippet to
// stdout: the command followed by a heredoc containing stdin, using a
// random heredoc terminator to avoid collisions with the payload. Pasting
// the output into a shell reproduces the effect of a live run.
func PipeTo(ctx context.Context, dryRun bool, stdin []byte, argv0 string, args ...string) error {
	env := testable.Env
	argv := append([]string{argv0}, args...)
	quoted := shellquote.Join(argv...)

	if dryRun {
		term := heredocTerminator()
		// Ensure the terminator sits at the start of its own line: add a
		// trailing newline if stdin does not already end with one.
		sep := ""
		if !bytes.HasSuffix(stdin, []byte("\n")) {
			sep = "\n"
		}
		_, err := fmt.Fprintf(env.Stdout, "%s <<'%s'\n%s%s%s\n", quoted, term, stdin, sep, term)
		return err
	}

	path, err := deps.LookPath(argv0)
	if err != nil {
		return err
	}

	logx.Command("%s", quoted)

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr

	return env.RunCommand(cmd)
}

// MustPipeTo is like [PipeTo] but logs and exits on failure.
func MustPipeTo(ctx context.Context, dryRun bool, stdin []byte, argv0 string, args ...string) {
	testable.Env.LogFatalOnError0(PipeTo(ctx, dryRun, stdin, argv0, args...))
}

// RunTolerant is like [Run] but suppresses non-zero exit codes, mirroring
// the shell idiom "cmd || true". Setup errors (command not in the allowlist,
// LookPath failure) are still reported.
//
// When dryRun is true, the rendered shell line includes a trailing "|| true".
func RunTolerant(ctx context.Context, dryRun bool, argv0 string, args ...string) error {
	env := testable.Env
	argv := append([]string{argv0}, args...)
	quoted := shellquote.Join(argv...)

	if dryRun {
		_, err := fmt.Fprintf(env.Stdout, "%s || true\n", quoted)
		return err
	}

	path, err := deps.LookPath(argv0)
	if err != nil {
		return err
	}

	logx.Command("%s || true", quoted)

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = env.Stdin
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr
	// TODO(bassosimone): this swallows every RunCommand error, not just
	// non-zero exits: failure to start (ENOENT, EACCES) and ctx
	// cancellation are silenced too, which is broader than the "|| true"
	// idiom we advertise (the shell still fails loud when the command
	// cannot be started). Consider suppressing only *exec.ExitError and
	// returning everything else.
	_ = env.RunCommand(cmd)

	return nil
}

// MustRunTolerant is like [RunTolerant] but logs and exits on failure.
func MustRunTolerant(ctx context.Context, dryRun bool, argv0 string, args ...string) {
	testable.Env.LogFatalOnError0(RunTolerant(ctx, dryRun, argv0, args...))
}

// heredocTerminator returns a fresh heredoc terminator with a 64-bit random
// suffix. A collision with the payload would require the caller to embed a
// matching NPTE_EOF_<hex> line on purpose, which has probability 2^-64 for
// any given call — low enough to ignore without an explicit check.
func heredocTerminator() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "NPTE_EOF_" + strings.ToUpper(hex.EncodeToString(b[:]))
}
