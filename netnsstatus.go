// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netnsStatusMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte netns status", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Check whether the network namespaces for a project are up. "+
			"Prints 'up' or 'down' to stdout and exits with code 0 (up) or 1 (down). "+
			"Useful in shell scripts:",
		"    npte netns status myproj || sudo npte netns up myproj.",
		"The <project> argument selects the project to check. "+
			"See 'npte tutorial namespaces' for details.",
	)
	usage.PositionalArgumentsUsage = "<project>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args)) // we are using vflag.ExitOnError

	proj := fset.Args()[0]
	if err := validateProject(proj); err != nil {
		logx.Error("npte netns status: %s", err)
		env.Exit(2)
	}

	if _, err := env.Stat(nsPath(proj, "router")); err != nil {
		fmt.Fprintln(env.Stdout, "down")
		env.Exit(1)
	}
	fmt.Fprintln(env.Stdout, "up")
	return nil
}
