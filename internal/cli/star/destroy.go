// SPDX-License-Identifier: GPL-3.0-or-later

package star

import (
	"context"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// destroyMain is the main of the `star destroy` subcommand.
func destroyMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte star destroy", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Undoes `npte star create`: destroys the `client`, `server`, and "+
			"`router` namespaces. The `netns destroy` calls cascade to the veth "+
			"pairs and per-namespace iptables state.",
		"Does NOT touch gateway state. If `npte gateway create router ...` "+
			"was layered on top of the star, run `npte gateway destroy router` "+
			"separately — order does not matter, since `gateway destroy` is "+
			"tolerant of an absent namespace and removes host-side state by "+
			"its `npte:gw:<ns>` tag.",
		"Takes no arguments: names and layout match `npte star create`. This "+
			"command is strict — a first-error exit surfaces partial state rather "+
			"than hiding it. To clean up manually, call `npte netns destroy` "+
			"directly.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run. The script sets no shell "+
			"options of its own; wrap it (e.g. with `set -euxo pipefail`) "+
			"if you want fail-fast semantics.",
	)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 0
	fset.MaxPositionalArgs = 0
	runtimex.PanicOnError0(fset.Parse(args))

	self := selfPath("npte star destroy")

	pass := func(argv ...string) []string {
		if dryRun {
			return append(argv, "-n")
		}
		return argv
	}

	logx.Details("npte: tear down star topology (client/router/server)")

	runSelf(ctx, self, pass("netns", "destroy", "client")...)
	runSelf(ctx, self, pass("netns", "destroy", "server")...)
	runSelf(ctx, self, pass("netns", "destroy", "router")...)

	logx.Details("npte: star topology is gone")
	return nil
}
