// SPDX-License-Identifier: GPL-3.0-or-later

package gateway

import (
	"context"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// destroyMain is the main of the `gateway destroy` subcommand.
func destroyMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte gateway destroy", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Undoes `npte gateway create`: removes all host-side iptables rules "+
			"tagged \"npte:gw:<ns>\" via an atomic iptables-save/restore pipeline, "+
			"and removes the host-side veth if it still exists.",
		"Safe to run in any order relative to `npte netns destroy`. If the "+
			"namespace was already destroyed, the host-side veth is gone (the "+
			"kernel cleans a veth when its far-end namespace disappears); the "+
			"veth-removal step tolerates that. If `npte gateway destroy` runs "+
			"first, `npte netns destroy` still works to clean up the namespace.",
		"Does NOT toggle net.ipv4.ip_forward back to 0. That setting is "+
			"host-wide and may be wanted by other things on the system (Docker, "+
			"other gateways, etc.); flipping it off would break them.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<ns>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	ns := fset.Args()[0]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte gateway destroy: %s", err)
		env.Exit(2)
		return nil
	}

	tag := "npte:gw:" + ns
	hostIface := "if-" + ns

	logx.Details("npte: remove host-side iptables rules tagged %q", tag)
	subprocess.MustPipeline(ctx, dryRun,
		[]string{"iptables-save"},
		[]string{"grep", "-Fv", tag},
		[]string{"iptables-restore"},
	)

	logx.Details("npte: remove host-side veth %q (may already be gone)", hostIface)
	subprocess.MustRunTolerant(ctx, dryRun, "ip", "link", "del", hostIface)

	logx.Details("npte: gateway state for %q is cleaned up", ns)
	return nil
}
