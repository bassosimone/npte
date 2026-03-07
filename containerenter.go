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

func containerEnterMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte container enter", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Enter a lightweight container using systemd-nspawn. Binds the container's "+
			"filesystem tree to the corresponding network namespace. "+
			"Any trailing arguments are passed to systemd-nspawn.",
		"The <project> argument selects the project. "+
			"The <name> argument is the name of the network namespace whose container to enter.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<project> <name> [nspawn-args...]"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	nameFlag := fset.Args()[1]

	// Load config and resolve namespace path
	logDetails("npte: load config from %s\n", configPath(proj))
	cfg := mustLoadNetnsConfig(proj)
	if err := validateEndpointName(cfg.Project, nameFlag); err != nil {
		logAlways("npte container enter: %s\n", err)
		env.Exit(2)
	}
	ns := nsName(proj, nameFlag)
	nsp := nsPath(proj, nameFlag)

	// Verify the filesystem tree exists
	tree := treePath(proj, nameFlag)
	if _, err := env.Stat(tree); os.IsNotExist(err) {
		logAlways("npte container enter: tree not found: %s\n", tree)
		logAlways("npte container enter: create it with `npte container create %s %s'\n", proj, nameFlag)
		env.Exit(1)
	}

	// Enter the container with systemd-nspawn
	logDetails("npte: enter container '%s' in namespace '%s'\n", nameFlag, ns)
	nspawnArgs := []string{"-D", tree, "--network-namespace-path=" + nsp}
	nspawnArgs = append(nspawnArgs, fset.Args()[2:]...)

	logDetails("npte: systemd-nspawn %s\n", strings.Join(nspawnArgs, " "))

	cmd := exec.CommandContext(ctx, "systemd-nspawn", nspawnArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env.LogFatalOnError0(env.RunCommand(cmd))
	return nil
}
