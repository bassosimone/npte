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

func netnsRunMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte netns run", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Run a command inside a network namespace. The command runs with the privileges "+
			"of the calling user (identified by $SUDO_USER), not as root.",
		"The <project> argument selects the project. "+
			"The <name> argument is the name of the network namespace to enter. "+
			"The <command> and optional [args...] are executed inside it.",
		"Example: sudo npte netns run myproj server curl https://example.com/",
		"This command must be run via sudo.",
	)
	usage.PositionalArgumentsUsage = "<project> <name> <command> [args...]"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 3
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	nameFlag := fset.Args()[1]

	// Load config and resolve namespace path
	logDetails("npte: load config from %s\n", configPath(proj))
	cfg := mustLoadNetnsConfig(proj)
	if err := validateEndpointName(cfg.Project, nameFlag); err != nil {
		logAlways("npte netns run: %s\n", err)
		env.Exit(2)
	}
	ns := nsName(proj, nameFlag)
	nsp := nsPath(proj, nameFlag)

	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		logAlways("npte netns run: $SUDO_USER is not set; this command must be run via sudo\n")
		env.Exit(1)
	}

	// Bernstein pipeline: nsenter enters the namespace, runuser drops privileges
	logDetails("npte: enter namespace '%s' as user '%s'\n", ns, sudoUser)
	nsenterArgs := []string{"--net=" + nsp, "--", "runuser", "-u", sudoUser, "--"}
	nsenterArgs = append(nsenterArgs, fset.Args()[2:]...)

	logDetails("npte: nsenter %s\n", strings.Join(nsenterArgs, " "))

	cmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env.LogFatalOnError0(env.RunCommand(cmd))
	return nil
}
