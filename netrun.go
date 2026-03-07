// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"math"
	"os"
	"os/exec"
	"strings"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netRunMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte net run", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Run a command inside a network namespace. The command runs with the privileges "+
			"of the calling user (identified by $SUDO_USER), not as root.",
		"The <name> argument is the name of the network namespace to enter. "+
			"The <command> and optional [args...] are executed inside it.",
		"This command must be run via sudo.",
	)
	usage.PositionalArgumentsUsage = "<name> <command> [args...]"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args))

	nameFlag := fset.Args()[0]

	// Load network state and resolve namespace path
	logDetails("npte: load network state from %s\n", netStatePath)
	state := mustLoadNetState()
	if err := validateEndpointName(state.Prefix, nameFlag); err != nil {
		logAlways("npte net run: %s\n", err)
		env.Exit(2)
	}
	pfx := state.Prefix
	ns := nsName(pfx, nameFlag)
	nsp := nsPath(pfx, nameFlag)

	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		logAlways("npte net run: $SUDO_USER is not set; this command must be run via sudo\n")
		env.Exit(1)
	}

	// Bernstein pipeline: nsenter enters the namespace, runuser drops privileges
	logDetails("npte: enter namespace '%s' as user '%s'\n", ns, sudoUser)
	nsenterArgs := []string{"--net=" + nsp, "--", "runuser", "-u", sudoUser, "--"}
	nsenterArgs = append(nsenterArgs, fset.Args()[1:]...)

	logDetails("npte: nsenter %s\n", strings.Join(nsenterArgs, " "))

	cmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env.LogFatalOnError0(env.RunCommand(cmd))
	return nil
}
