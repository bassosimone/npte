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
	var nameFlag string

	fset := vflag.NewFlagSet("npte container enter", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Enter a lightweight container using systemd-nspawn. Binds the container's "+
			"filesystem tree to the corresponding network namespace. Any extra arguments "+
			"after '--' are passed to systemd-nspawn.",
		"This command must be run as root (e.g., via sudo).",
	)
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&nameFlag, 'n', "name", "The `NAME` of the host.")
	fset.MaxPositionalArgs = math.MaxInt
	runtimex.PanicOnError0(fset.Parse(args))

	if nameFlag == "" {
		fmt.Fprintf(env.Stderr, "npte container enter: --name is required\n")
		fmt.Fprintf(env.Stderr, "npte container enter: try `npte container enter --help' for more help.\n")
		env.Exit(2)
	}

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
		fmt.Fprintf(env.Stderr, "npte container enter: create it with `npte container create -n %s'\n", nameFlag)
		env.Exit(1)
	}

	// Enter the container with systemd-nspawn
	fmt.Fprintf(env.Stderr, "npte: enter container '%s' in namespace '%s'\n", nameFlag, ns)
	nspawnArgs := []string{"-D", tree, "--network-namespace-path=" + nsp}
	nspawnArgs = append(nspawnArgs, fset.Args()...)

	fmt.Fprintf(env.Stderr, "npte: systemd-nspawn %s\n", strings.Join(nspawnArgs, " "))

	cmd := exec.CommandContext(ctx, "systemd-nspawn", nspawnArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env.LogFatalOnError0(env.RunCommand(cmd))
	return nil
}
