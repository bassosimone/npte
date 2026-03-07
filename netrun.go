// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netRunMain(ctx context.Context, args []string) error {
	// Parse command line flags
	var nameFlag string

	fset := vflag.NewFlagSet("npte net run", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Run a command inside a network namespace. Uses nsenter to enter the namespace "+
			"and runuser to drop back to the calling user's privileges.",
		"This command must be run via sudo.",
	)
	usage.PositionalArgumentsUsage = "command [args...]"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&nameFlag, 'n', "name", "The `NAME` of the host.")
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args))

	if nameFlag == "" || len(fset.Args()) <= 0 {
		fmt.Fprintf(os.Stderr, "npte net run: --name and a command after '--' are required\n")
		fmt.Fprintf(os.Stderr, "npte net run: try `npte net run --help' for more help.\n")
		os.Exit(2)
	}

	// Load network state and resolve namespace path
	fmt.Fprintf(os.Stderr, "npte: load network state from %s\n", netStatePath)
	state := mustLoadNetState()
	if err := validateEndpointName(state.Prefix, nameFlag); err != nil {
		fmt.Fprintf(os.Stderr, "npte net run: %s\n", err)
		os.Exit(2)
	}
	pfx := state.Prefix
	ns := nsName(pfx, nameFlag)
	nsp := nsPath(pfx, nameFlag)

	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		fmt.Fprintf(os.Stderr, "npte net run: $SUDO_USER is not set; this command must be run via sudo\n")
		os.Exit(1)
	}

	// Bernstein pipeline: nsenter enters the namespace, runuser drops privileges
	fmt.Fprintf(os.Stderr, "npte: enter namespace '%s' as user '%s'\n", ns, sudoUser)
	nsenterArgs := []string{"--net=" + nsp, "--", "runuser", "-u", sudoUser, "--"}
	nsenterArgs = append(nsenterArgs, fset.Args()...)

	fmt.Fprintf(os.Stderr, "npte: nsenter %s\n", strings.Join(nsenterArgs, " "))

	cmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runtimex.LogFatalOnError0(cmd.Run())
	return nil
}
