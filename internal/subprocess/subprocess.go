// SPDX-License-Identifier: GPL-3.0-or-later

// Package subprocess contains code to exec subprocesses.
package subprocess

import (
	"context"
	"fmt"
	"os/exec"

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
