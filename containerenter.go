// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func containerEnterMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte container enter", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Enter a lightweight container using systemd-nspawn. Binds the container's "+
			"filesystem tree to the corresponding network namespace. Any extra arguments "+
			"after <name> are passed to systemd-nspawn.",
		"The <name> argument is the name of the network namespace whose container to enter.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<name> [nspawn-args...]"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args))

	nameFlag := fset.Args()[0]

	// Load network state and resolve namespace path
	fmt.Fprintf(env.Stderr, "npte: load network state from %s\n", netStatePath)
	state := mustLoadNetState()
	if err := validateEndpointName(state.Prefix, nameFlag); err != nil {
		fmt.Fprintf(env.Stderr, "npte container enter: %s\n", err)
		env.Exit(2)
	}
	pfx := state.Prefix
	ns := nsName(pfx, nameFlag)
	nsp := nsPath(pfx, nameFlag)

	// Verify the filesystem tree exists
	tree := filepath.Join(".npte", "trees", nameFlag)
	if _, err := env.Stat(tree); os.IsNotExist(err) {
		fmt.Fprintf(env.Stderr, "npte container enter: tree not found: %s\n", tree)
		fmt.Fprintf(env.Stderr, "npte container enter: create it with `npte container create %s'\n", nameFlag)
		env.Exit(1)
	}

	// Enter the container with systemd-nspawn
	fmt.Fprintf(env.Stderr, "npte: enter container '%s' in namespace '%s'\n", nameFlag, ns)
	nspawnArgs := []string{"-D", tree, "--network-namespace-path=" + nsp}
	nspawnArgs = append(nspawnArgs, fset.Args()[1:]...)

	fmt.Fprintf(env.Stderr, "npte: systemd-nspawn %s\n", strings.Join(nspawnArgs, " "))

	cmd := exec.CommandContext(ctx, "systemd-nspawn", nspawnArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env.LogFatalOnError0(env.RunCommand(cmd))
	return nil
}
