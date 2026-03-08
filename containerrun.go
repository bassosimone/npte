// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
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
		"Run a command inside a lightweight container using systemd-nspawn "+
			"using the network namespace identified by <name>. "+
			"If the optional [command] is omitted, we spawn an interactive shell.",
		"The <project> argument selects the project. ",
		"This command requires root privileges (e.g., via sudo). "+
			"See 'npte tutorial containers' for details.",
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

	if err := validateProject(proj); err != nil {
		logError("npte container run: %s", err)
		env.Exit(2)
	}

	// Load config and resolve namespace path
	logDetails("npte: load config from %s", configPath(proj))
	cfg := mustLoadNetnsConfig(proj)
	if err := validateEndpointName(cfg.Project, nameFlag); err != nil {
		logError("npte container run: %s", err)
		env.Exit(2)
	}
	ns := nsName(proj, nameFlag)
	nsp := nsPath(proj, nameFlag)

	// Verify the filesystem tree exists
	tree := treePath(proj, nameFlag)
	if _, err := env.Stat(tree); os.IsNotExist(err) {
		logError("npte container run: tree not found: %s", tree)
		logError("npte container run: create it with %q", fmt.Sprintf("npte container create %s %s", proj, nameFlag))
		env.Exit(1)
	}

	// Run inside the container with systemd-nspawn
	logDetails("npte: run in container %q in namespace %q", nameFlag, ns)
	nspawnArgs := []string{"-D", tree, "--network-namespace-path=" + nsp}
	nspawnArgs = append(nspawnArgs, fset.Args()[2:]...)

	mustRunArgs(ctx, "systemd-nspawn", nspawnArgs...)
	return nil
}
