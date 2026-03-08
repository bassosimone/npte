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
	var prefixFlag = defaultPrefix

	fset := vflag.NewFlagSet("npte project create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Create the directory skeleton and initial configuration for a new project. "+
			"This creates the config and trees directories under "+baseDir+"/<name>/ "+
			"and writes the initial network configuration file at "+
			baseDir+"/<name>/config/netns.json",
		"The <name> argument is the project name.",
		"Use --prefix to assign a different /16 block when running multiple projects "+
			"(default: "+defaultPrefix+").",
		"This command requires root privileges (e.g., via sudo). "+
			"See 'npte tutorial quickstart' for a complete walkthrough.",
	)
	usage.PositionalArgumentsUsage = "<name>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&prefixFlag, 0, "prefix", "The /16 address `PREFIX` for this project (default: "+defaultPrefix+").")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	// Get the project directory
	proj := fset.Args()[0]
	if err := validateProject(proj); err != nil {
		logError("npte project create: %s", err)
		env.Exit(2)
	}

	// Validate prefix
	if _, err := validatePrefix(prefixFlag); err != nil {
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

	// Write initial config with prefix
	cfg := &netnsConfig{
		Prefix:          prefixFlag,
		NextSubnetIndex: 1,
		Hosts:           make(map[string]*hostConfig),
	}
	logDetails("npte: write initial config with prefix %s", prefixFlag)
	env.LogFatalOnError0(saveNetnsConfig(proj, cfg))

	logDetails("npte: project %q created", proj)
	return nil
}
