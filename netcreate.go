// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netCreateMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte net create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Add a network namespace to the project configuration. "+
			"Automatically allocates a /24 subnet for the new namespace. "+
			"This command only modifies the config file; use 'npte net up' to create the actual namespaces.",
		"The <project> argument selects the project. "+
			"The <name> argument is the name of the network namespace to add.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<project> <name>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = 2
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	nameFlag := fset.Args()[1]

	if err := validateProject(proj); err != nil {
		logAlways("npte net create: %s\n", err)
		env.Exit(2)
	}
	if err := validateEndpointName(proj, nameFlag); err != nil {
		logAlways("npte net create: %s\n", err)
		env.Exit(2)
	}

	// Verify the project directory exists
	if _, err := env.Stat(projectDir(proj)); os.IsNotExist(err) {
		logAlways("npte net create: project %q not found\n", proj)
		logAlways("npte net create: run `npte project create %s' first\n", proj)
		env.Exit(1)
	}

	unlock := mustLockConfig(proj)
	defer unlock()

	// Load existing config or create a new one
	cfg, err := loadConfig(proj)
	if err != nil {
		if !os.IsNotExist(err) {
			logAlways("npte net create: cannot load config: %s\n", err)
			env.Exit(1)
		}
		cfg = &netConfig{
			Project:         proj,
			NextSubnetIndex: 1,
			Hosts:           make(map[string]*hostConfig),
		}
	}

	if _, exists := cfg.Hosts[nameFlag]; exists {
		logAlways("npte net create: host %q already exists\n", nameFlag)
		env.Exit(1)
	}

	// Allocate subnet
	subnet := fmt.Sprintf("10.0.%d.0/24", cfg.NextSubnetIndex)
	logDetails("npte: allocate subnet %s for host %q\n", subnet, nameFlag)

	cfg.Hosts[nameFlag] = &hostConfig{
		Name:   nameFlag,
		Subnet: subnet,
	}
	cfg.NextSubnetIndex++

	logDetails("npte: save config to %s\n", configPath(proj))
	env.LogFatalOnError0(saveConfig(proj, cfg))

	logDetails("npte: added host %q to config (subnet %s)\n", nameFlag, subnet)
	logDetails("npte: run `npte net up %s' to create the namespaces\n", proj)
	return nil
}
