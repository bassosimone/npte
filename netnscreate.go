// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netnsCreateMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte netns create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Add a network namespace to the project configuration. "+
			"Automatically allocates a /24 subnet within the project's prefix.",
		"This command only modifies the config file; use 'npte netns up' to create the actual network namespaces.",
		"The <project> argument selects the project. "+
			"The <name> argument is the name of the network namespace to add.",
		"This command requires root privileges (e.g., via sudo). "+
			"See 'npte tutorial quickstart' for a complete walkthrough.",
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
		logError("npte netns create: %s", err)
		env.Exit(2)
	}
	if err := validateEndpointName(proj, nameFlag); err != nil {
		logError("npte netns create: %s", err)
		env.Exit(2)
	}

	// Verify the project directory exists
	if _, err := env.Stat(projectDir(proj)); os.IsNotExist(err) {
		logError("npte netns create: project %q not found", proj)
		logError("npte netns create: run %q first", fmt.Sprintf("npte project create %s", proj))
		env.Exit(1)
	}

	unlock := mustLockNetnsConfig(proj)
	defer unlock()

	// Load existing config
	cfg := mustLoadNetnsConfig(proj)

	if _, exists := cfg.Hosts[nameFlag]; exists {
		logError("npte netns create: host %q already exists", nameFlag)
		env.Exit(1)
	}

	// Allocate subnet index
	index := cfg.NextSubnetIndex
	subnet := cfg.mustSubnet(index)
	logDetails("npte: allocate subnet %s for host %q", subnet, nameFlag)

	cfg.Hosts[nameFlag] = &hostConfig{
		Name:        nameFlag,
		SubnetIndex: index,
	}
	cfg.NextSubnetIndex++

	logDetails("npte: save config to %s", configPath(proj))
	env.LogFatalOnError0(saveNetnsConfig(proj, cfg))

	logDetails("npte: added host %q to config (subnet %s)", nameFlag, subnet)
	logDetails("npte: run %q to create the network namespaces", fmt.Sprintf("npte netns up %s", proj))
	return nil
}
