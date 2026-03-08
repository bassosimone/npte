// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"math"
	"os"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func containerRunMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte container run", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Run a command inside a lightweight container using systemd-nspawn. "+
			"Binds the container's filesystem tree to the corresponding network namespace. "+
			"Without arguments, spawns an interactive shell.",
		"The <project> argument selects the project. "+
			"The <name> argument is the name of the network namespace whose container to use.",
		"Example: sudo npte container run myproj server nginx -g 'daemon off;'",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<project> <name> [command] [args...]"
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
		logAlways("npte container run: %s\n", err)
		env.Exit(2)
	}
	ns := nsName(proj, nameFlag)
	nsp := nsPath(proj, nameFlag)

	// Verify the filesystem tree exists
	tree := treePath(proj, nameFlag)
	if _, err := env.Stat(tree); os.IsNotExist(err) {
		logAlways("npte container run: tree not found: %s\n", tree)
		logAlways("npte container run: create it with `npte container create %s %s'\n", proj, nameFlag)
		env.Exit(1)
	}

	// Run inside the container with systemd-nspawn
	logDetails("npte: run in container '%s' in namespace '%s'\n", nameFlag, ns)
	nspawnArgs := []string{"-D", tree, "--network-namespace-path=" + nsp}
	nspawnArgs = append(nspawnArgs, fset.Args()[2:]...)

	mustRunArgs(ctx, "systemd-nspawn", nspawnArgs...)
	return nil
}
