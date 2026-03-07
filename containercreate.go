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

func containerCreateMain(ctx context.Context, args []string) error {
	// Parse command line flags
	var (
		nameFlag  string
		suiteFlag = "noble"
	)

	fset := vflag.NewFlagSet("npte container create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Create a lightweight container filesystem tree using debootstrap. "+
			"The tree is stored under .npte/trees/<name> and can later be entered "+
			"with 'npte container enter'.",
		"This command must be run as root (e.g., via sudo).",
	)
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&nameFlag, 'n', "name", "The `NAME` of the host.")
	fset.StringVar(&suiteFlag, 's', "suite", "The distribution `SUITE` to bootstrap (default: noble).")
	runtimex.PanicOnError0(fset.Parse(args))

	if nameFlag == "" {
		fmt.Fprintf(os.Stderr, "npte container create: --name is required\n")
		fmt.Fprintf(os.Stderr, "npte container create: try `npte container create --help' for more help.\n")
		os.Exit(2)
	}

	pfx := mustLoadPrefix()
	if err := validateEndpointName(pfx, nameFlag); err != nil {
		fmt.Fprintf(os.Stderr, "npte container create: %s\n", err)
		os.Exit(2)
	}

	tree := filepath.Join(".npte", "trees", nameFlag)
	if _, err := os.Stat(tree); err == nil {
		fmt.Fprintf(os.Stderr, "npte container create: tree already exists: %s\n", tree)
		os.Exit(1)
	}

	// Bootstrap the filesystem tree
	fmt.Fprintf(os.Stderr, "npte: bootstrap '%s' filesystem tree at %s\n", suiteFlag, tree)
	fmt.Fprintf(os.Stderr, "npte: ensure parent directory exists\n")
	fmt.Fprintf(os.Stderr, "+ mkdir -p %s\n", filepath.Dir(tree))
	runtimex.LogFatalOnError0(os.MkdirAll(filepath.Dir(tree), 0755))

	fmt.Fprintf(os.Stderr, "npte: run debootstrap (this may take a while)\n")
	mustRun("debootstrap %s %s", suiteFlag, tree)

	fmt.Fprintf(os.Stderr, "npte: container tree created at %s\n", tree)
	return nil
}
