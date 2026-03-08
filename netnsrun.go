// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"math"
	"os"
	"strings"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netnsRunMain(ctx context.Context, args []string) error {
	// Parse command line flags
	var userFlag = os.Getenv("SUDO_USER")
	var envFlags []string

	fset := vflag.NewFlagSet("npte netns run", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Run a command inside a network namespace. By default, the command runs "+
			"as $SUDO_USER. Use -u/--user to override (e.g., --user root for admin tasks like tc). "+
			"Use -e/--env to set environment variables.",
		"The <project> argument selects the project. "+
			"The <name> argument is the name of the network namespace to use. "+
			"The <command> and optional [args...] are executed inside it.",
		"This command requires root privileges (e.g., via sudo) and uses $SUDO_USER "+
			"to determine which user to run as, unless you use -u/--user. "+
			"See 'npte tutorial namespaces' for details.",
	)
	usage.PositionalArgumentsUsage = "<project> <name> <command> [args...]"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&userFlag, 'u', "user", "Run as `USER` (default: $SUDO_USER).")
	fset.StringSliceVar(&envFlags, 'e', "env", "Set environment variable `KEY=VALUE` (repeatable).")
	fset.MinPositionalArgs = 3
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	nameFlag := fset.Args()[1]

	// Load config and resolve namespace path
	logDetails("npte: load config from %s", configPath(proj))
	cfg := mustLoadNetnsConfig(proj)
	if err := validateEndpointName(cfg.Project, nameFlag); err != nil {
		logError("npte netns run: %s", err)
		env.Exit(2)
	}
	ns := nsName(proj, nameFlag)
	nsp := nsPath(proj, nameFlag)

	if userFlag == "" {
		logError("npte netns run: no user specified; use --user or run via sudo")
		env.Exit(1)
	}

	// Validate -e flags
	for _, ev := range envFlags {
		if !strings.Contains(ev, "=") {
			logError("npte netns run: -e value must be KEY=VALUE, got %q", ev)
			env.Exit(2)
		}
	}

	// Use systemd-run to:
	// 1. enter the network namespace (NetworkNamespacePath)
	// 2. overlay the project's resolv.conf (BindPaths)
	// 3. drop privileges to the specified user (--uid)
	// 4. set environment variables (--setenv)
	logDetails("npte: enter namespace %q as user %q", ns, userFlag)
	rc := resolvConfPath(proj)
	sdArgs := []string{
		"--pipe", "--quiet", "--collect",
		"--property=NetworkNamespacePath=" + nsp,
		"--property=BindPaths=" + rc + ":/etc/resolv.conf",
		"--uid=" + userFlag,
	}
	for _, ev := range envFlags {
		sdArgs = append(sdArgs, "--setenv="+ev)
	}
	sdArgs = append(sdArgs, "--")
	sdArgs = append(sdArgs, fset.Args()[2:]...)

	mustRunArgs(ctx, "systemd-run", sdArgs...)
	return nil
}
