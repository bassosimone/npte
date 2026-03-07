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
			"The tree is stored under .npte/trees/<name> and can later be entered "+
			"with 'npte container enter'.",
		"The <name> argument is the name of the network namespace whose filesystem tree to create.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<name>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&suiteFlag, 0, "suite", "The distribution `SUITE` to bootstrap (default: noble).")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	nameFlag := fset.Args()[0]

	pfx := mustLoadPrefix()
	if err := validateEndpointName(pfx, nameFlag); err != nil {
		logAlways("npte container create: %s\n", err)
		env.Exit(2)
	}

	tree := filepath.Join(".npte", "trees", nameFlag)
	if _, err := env.Stat(tree); err == nil {
		logAlways("npte container create: tree already exists: %s\n", tree)
		env.Exit(1)
	}

	// Bootstrap the filesystem tree
	logDetails("npte: bootstrap '%s' filesystem tree at %s\n", suiteFlag, tree)
	logDetails("npte: ensure parent directory exists\n")
	logDetails("+ mkdir -p %s\n", filepath.Dir(tree))
	env.LogFatalOnError0(env.MkdirAll(filepath.Dir(tree), 0755))

	logDetails("npte: run debootstrap (this may take a while)\n")
	mustRun("debootstrap %s %s", suiteFlag, tree)

	logDetails("npte: container tree created at %s\n", tree)
	return nil
}
