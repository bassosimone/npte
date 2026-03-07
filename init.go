// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func initMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte init", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Initialize or reinitialize the '.npte' directory in the current working directory. " +
			"Creates the required subdirectories (.npte/config, .npte/state, .npte/trees) " +
			"and writes the default configuration file if it does not already exist.",
	)
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	runtimex.PanicOnError0(fset.Parse(args))

	// Create the required directories if needed
	fmt.Fprintf(env.Stderr, "npte: initialize or reinitialize the local '.npte' dir\n")
	fmt.Fprintf(env.Stderr, "npte: ensure the required directories exist\n")
	dirs := []string{
		filepath.Join(".npte", "config"),
		filepath.Join(".npte", "state"),
		filepath.Join(".npte", "trees"),
	}
	for _, dir := range dirs {
		fmt.Fprintf(env.Stderr, "+ mkdir -p %s\n", dir)
		env.LogFatalOnError0(env.MkdirAll(dir, 0755))
	}

	// Write the .npte/config/name if needed
	namePath := filepath.Join(".npte", "config", "name")
	if _, err := env.Stat(namePath); os.IsNotExist(err) {
		fmt.Fprintf(env.Stderr, "npte: creating default config file: %s\n", namePath)
		env.LogFatalOnError0(env.WriteFile(namePath, []byte("npte\n"), 0644))
	}

	fmt.Fprintf(env.Stderr, "npte: done\n")
	return nil
}
