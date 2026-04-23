// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/runtimex"
	"github.com/kballard/go-shellquote"
)

func runArgs(ctx context.Context, argv0 string, args ...string) error {
	argv := append([]string{argv0}, args...)
	logx.Command("%s", shellquote.Join(argv...))

	cmd := exec.CommandContext(ctx, argv0, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return env.RunCommand(cmd)
}

func mustRunArgs(ctx context.Context, argv0 string, args ...string) {
	env.LogFatalOnError0(runArgs(ctx, argv0, args...))
}

func runCmd(ctx context.Context, format string, args ...any) error {
	cmdline := fmt.Sprintf(format, args...)
	argv, err := shellquote.Split(cmdline)
	if err != nil {
		return err
	}
	runtimex.Assert(len(argv) > 0)
	logx.Command("%s", cmdline)

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return env.RunCommand(cmd)
}

func mustRunCmd(ctx context.Context, format string, args ...any) {
	env.LogFatalOnError0(runCmd(ctx, format, args...))
}
