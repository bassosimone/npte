// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"path/filepath"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func projectCreateMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte project create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Create the directory skeleton for a new project. "+
			"This creates the config and trees directories under "+baseDir+"/<name>/.",
		"The <name> argument is the project name.",
		"After creating the project, add hosts with 'npte netns create' "+
			"and bring the network up with 'npte netns up'.",
		"This command must be run as root (e.g., using sudo).",
	)
	usage.PositionalArgumentsUsage = "<name>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	// Get the project directory
	proj := fset.Args()[0]
	if err := validateProject(proj); err != nil {
		logError("npte project create: %s", err)
		env.Exit(2)
	}

	pd := projectDir(proj)
	if _, err := env.Stat(pd); err == nil {
		logError("npte project create: project already exists: %s", pd)
		env.Exit(1)
	}

	// Populate the project directory
	logDetails("npte: create project skeleton at %s", pd)
	logCommand("mkdir -p %s", filepath.Join(pd, "config"))
	env.LogFatalOnError0(env.MkdirAll(filepath.Join(pd, "config"), 0755))
	logCommand("mkdir -p %s", filepath.Join(pd, "trees"))
	env.LogFatalOnError0(env.MkdirAll(filepath.Join(pd, "trees"), 0755))

	rc := resolvConfPath(proj)
	logDetails("npte: write default resolv.conf at %s", rc)
	env.LogFatalOnError0(env.WriteFile(rc, []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n"), 0644))

	logDetails("npte: project %q created", proj)
	return nil
}
