// SPDX-License-Identifier: GPL-3.0-or-later

package star

import (
	"context"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// createMain is the main of the `star create` subcommand.
func createMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte star create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Creates a three-node star topology. Two leaf namespaces, `client` and "+
			"`server`, each share a veth pair with a hub namespace, `router`. Both "+
			"leaves get a default route via the router. Off-link traffic is not "+
			"wired up: the leaves can talk to each other through the router, but "+
			"there is no host uplink and no NAT. To give the star internet egress, "+
			"layer `npte gateway create router 172.16.1.0/24 <ext-iface>` on top.",
		"Addresses are fixed, and drawn from 172.16.0.0/16 — the quietest "+
			"corner of RFC1918 in practice (home/ISP CPE prefer 192.168/16 or "+
			"10/8; Docker's default bridge sits in 172.17.0.0/16, one /16 "+
			"over). The second octet tags the link:",
		"    server↔router uses 172.16.2.0/24 (server=.2, router=.1)",
		"    client↔router uses 172.16.3.0/24 (client=.2, router=.1)",
		"    router↔host   uses 172.16.1.0/24 — reserved for `npte gateway "+
			"create`, not wired by `star create` itself.",
		"Implementation is a composition of `npte netns create`, `npte netns "+
			"connect`, `npte netns assign-addr`, and `npte netns add-route`. For "+
			"non-default names, subnets, or shapes, call those primitives directly.",
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

	self := selfPath("npte star create")

	// Propagate --dry-run to each child invocation so the composed output
	// is a contiguous shell script mirroring what a live run would do.
	// The flag is appended (not prepended) so it lands in the leaf
	// subcommand's flagset rather than the root dispatcher's.
	pass := func(argv ...string) []string {
		if dryRun {
			return append(argv, "-n")
		}
		return argv
	}

	logx.Details("npte: compose star topology (client/router/server), no internet egress")

	runSelf(ctx, self, pass("netns", "create", "client")...)
	runSelf(ctx, self, pass("netns", "create", "router")...)
	runSelf(ctx, self, pass("netns", "create", "server")...)

	runSelf(ctx, self, pass("netns", "connect", "client", "router")...)
	runSelf(ctx, self, pass("netns", "connect", "server", "router")...)

	runSelf(ctx, self, pass("netns", "assign-addr", "client", "if-router", "172.16.3.2/24")...)
	runSelf(ctx, self, pass("netns", "assign-addr", "router", "if-client", "172.16.3.1/24")...)
	runSelf(ctx, self, pass("netns", "assign-addr", "server", "if-router", "172.16.2.2/24")...)
	runSelf(ctx, self, pass("netns", "assign-addr", "router", "if-server", "172.16.2.1/24")...)

	runSelf(ctx, self, pass("netns", "add-route", "client", "default", "172.16.3.1")...)
	runSelf(ctx, self, pass("netns", "add-route", "server", "default", "172.16.2.1")...)

	logx.Details("npte: star topology is up (leaf↔leaf only)")
	logx.Details("npte: for internet egress, run: npte gateway create router 172.16.1.0/24 <ext-iface>")
	return nil
}
