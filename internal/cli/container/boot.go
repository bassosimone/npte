// SPDX-License-Identifier: GPL-3.0-or-later

package container

import (
	"context"
	"path/filepath"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// bootMain is the main of the `container boot` subcommand.
//
// Execution strategy: `systemd-nspawn --boot -D <rootfs>
// [--network-namespace-path=/run/netns/<ns>]`.
//
// This is a separate subcommand from `container run` because booting has
// substantially different semantics: PID 1 inside the container is systemd
// (not a user command), exit is via three Ctrl+] presses, and the inner
// unit manager starts services from the tree's default target. That mode
// doesn't accept a trailing command, so fusing it into `run` would muddy
// the interface. Keeping them distinct matches the "small orthogonal
// primitives" style of the rest of the CLI.
func bootMain(ctx context.Context, args []string) error {
	env := testable.Env

	var (
		dryRun bool
		netns  string
	)

	fset := vflag.NewFlagSet("npte container boot", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Boots <rootfs> as a systemd-nspawn machine: systemd runs as PID 1 "+
			"inside the container and starts the tree's default target. Useful "+
			"when you need multiple services (nginx + postgres + ...) running "+
			"together under an init manager.",
		"If --netns is given, the container enters that network namespace; "+
			"otherwise it shares the host network namespace.",
		"To exit the container cleanly, press Ctrl+] three times.",
		"<rootfs> must be an absolute path to a populated filesystem tree "+
			"(e.g. one created with `npte container create`).",
		"With --dry-run, prints a round-trippable shell line to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<rootfs>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell line instead of executing it.")
	fset.StringVar(&netns, 0, "netns", "Enter network namespace `NS` (default: share the host's).")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	rootfs := fset.Args()[0]
	if !filepath.IsAbs(rootfs) {
		logx.Error("npte container boot: rootfs %q must be an absolute path", rootfs)
		env.Exit(2)
		return nil
	}
	if netns != "" {
		if err := validate.NetnsName(netns); err != nil {
			logx.Error("npte container boot: %s", err)
			env.Exit(2)
			return nil
		}
	}

	argv := []string{"--boot", "-D", rootfs}
	if netns != "" {
		argv = append(argv, "--network-namespace-path=/run/netns/"+netns)
	}

	if netns != "" {
		logx.Details("npte: boot %s inside namespace %q", rootfs, netns)
	} else {
		logx.Details("npte: boot %s in host network namespace", rootfs)
	}
	subprocess.MustRun(ctx, dryRun, "systemd-nspawn", argv...)
	return nil
}
