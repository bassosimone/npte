// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"path/filepath"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func containerCreateMain(ctx context.Context, args []string) error {
	// Parse command line flags
	var suiteFlag = "noble"

	fset := vflag.NewFlagSet("npte container create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Create a lightweight container filesystem tree using debootstrap. "+
			"The tree is stored under "+baseDir+"/<project>/trees/<name> and can later be entered "+
			"with 'npte container enter'.",
		"The <project> argument selects the project. "+
			"The <name> argument is the name of the network namespace whose filesystem tree to create. "+
			"The namespace must already exist (created via 'npte netns create').",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<project> <name>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&suiteFlag, 0, "suite", "The distribution `SUITE` to bootstrap (default: noble).")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = 2
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	nameFlag := fset.Args()[1]

	if err := validateProject(proj); err != nil {
		logError("npte container create: %s", err)
		env.Exit(2)
	}
	if err := validateEndpointName(proj, nameFlag); err != nil {
		logError("npte container create: %s", err)
		env.Exit(2)
	}

	tree := treePath(proj, nameFlag)
	if _, err := env.Stat(tree); err == nil {
		logError("npte container create: tree already exists: %s", tree)
		env.Exit(1)
	}

	// Bootstrap the filesystem tree
	logDetails("npte: bootstrap '%s' filesystem tree at %s", suiteFlag, tree)
	logDetails("npte: ensure parent directory exists")
	logCommand("mkdir -p %s", filepath.Dir(tree))
	env.LogFatalOnError0(env.MkdirAll(filepath.Dir(tree), 0755))

	logDetails("npte: run debootstrap (this may take a while)")
	mustRunCmd(ctx, "debootstrap %s %s", suiteFlag, tree)

	logDetails("npte: container tree created at %s", tree)
	return nil
}
