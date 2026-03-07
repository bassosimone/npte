// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bassosimone/runtimex"
	"github.com/kballard/go-shellquote"
)

func run(format string, args ...any) error {
	cmdline := fmt.Sprintf(format, args...)
	argv, err := shellquote.Split(cmdline)
	if err != nil {
		return err
	}
	runtimex.Assert(len(argv) > 0)
	logDetails("+ %s\n", cmdline)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return env.RunCommand(cmd)
}

func mustRun(format string, args ...any) {
	env.LogFatalOnError0(run(format, args...))
}
