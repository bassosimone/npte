// SPDX-License-Identifier: GPL-3.0-or-later

package container

import (
	"context"
	"math"
	"path/filepath"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// runMain is the main of the `container run` subcommand.
//
// Execution strategy: `systemd-nspawn -D <rootfs>
// [--network-namespace-path=/run/netns/<ns>] [--] [cmd args...]`.
//
// Why systemd-nspawn rather than chroot + ip netns exec: nspawn gives a
// proper mount namespace (each invocation sees its own /proc, /sys, /dev),
// an isolated PID namespace (the inner process is PID 1 of its own tree,
// so signal and cleanup semantics are well-defined), and the
// --network-namespace-path flag wires an existing netns in without
// requiring nspawn to manage one.
//
// Network namespace handling is orthogonal: when --netns is omitted the
// container shares the host network namespace, which is the correct
// default for "I just need a clean rootfs to install packages" flows.
// When --netns is given, the namespace must already exist (create it
// with `npte netns create`).
//
// Booting is a separate concern; see `npte container boot`.
func runMain(ctx context.Context, args []string) error {
	env := testable.Env

	var (
		dryRun       bool
		netns        string
		capabilities []string
		binds        []string
	)

	fset := vflag.NewFlagSet("npte container run", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Runs a command inside a systemd-nspawn container backed by <rootfs>. "+
			"If [command] is omitted, spawns an interactive shell. If --netns is "+
			"given, the container enters that network namespace; otherwise it "+
			"shares the host network namespace.",
		"Processes inside the container run as root in the container's own user "+
			"database, which is unrelated to the host's. To exit the container "+
			"cleanly, press Ctrl+] three times.",
		"Use --capability to add capabilities to nspawn's default bounding set "+
			"(e.g. --capability CAP_NET_ADMIN for OpenVPN). Use --bind to expose "+
			"host paths or device nodes inside the container (e.g. --bind "+
			"/dev/net/tun). Both flags are repeatable and their values are passed "+
			"verbatim to systemd-nspawn; see `man systemd-nspawn` for grammar.",
		"<rootfs> must be an absolute path to a populated filesystem tree "+
			"(e.g. one created with `npte container create`).",
		"With --dry-run, prints a round-trippable shell line to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<rootfs> [command] [args...]"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell line instead of executing it.")
	fset.StringVar(&netns, 0, "netns", "Enter network namespace `NS` (default: share the host's).")
	fset.StringSliceVar(&capabilities, 0, "capability", "Pass-through to nspawn `--capability=VALUE` (repeatable).")
	fset.StringSliceVar(&binds, 0, "bind", "Pass-through to nspawn `--bind=VALUE` (repeatable).")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args)) // cannot fail: using vflag.ExitOnError

	rootfs := fset.Args()[0]
	if !filepath.IsAbs(rootfs) {
		logx.Error("npte container run: rootfs %q must be an absolute path", rootfs)
		env.Exit(2)
		return nil
	}
	if netns != "" {
		if err := validate.NetnsName(netns); err != nil {
			logx.Error("npte container run: %s", err)
			env.Exit(2)
			return nil
		}
	}

	argv := []string{"-D", rootfs}
	if netns != "" {
		argv = append(argv, "--network-namespace-path=/run/netns/"+netns)
	}
	for _, c := range capabilities {
		argv = append(argv, "--capability="+c)
	}
	for _, b := range binds {
		argv = append(argv, "--bind="+b)
	}
	if len(fset.Args()) > 1 {
		argv = append(argv, "--")
		argv = append(argv, fset.Args()[1:]...)
	}

	if netns != "" {
		logx.Details("npte: nspawn %s inside namespace %q", rootfs, netns)
	} else {
		logx.Details("npte: nspawn %s in host network namespace", rootfs)
	}
	subprocess.MustRun(ctx, dryRun, "systemd-nspawn", argv...)
	return nil
}
