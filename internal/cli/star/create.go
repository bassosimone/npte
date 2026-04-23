// SPDX-License-Identifier: GPL-3.0-or-later

package star

import (
	"context"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// createMain is the main of the `star create` subcommand.
func createMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte star create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Creates a three-node star topology and wires the hub as an internet "+
			"gateway. Two leaf namespaces, `client` and `server`, each share a "+
			"veth pair with a hub namespace, `router`. Both leaves get a default "+
			"route via the router. The router gets a host-side uplink and NAT so "+
			"that traffic from the leaves egresses through <ext-iface>.",
		"Addresses are fixed, and drawn from 172.16/12 — the RFC1918 block "+
			"least likely to collide with existing home/ISP/VPN/container "+
			"networks. The second octet tags the link:",
		"    router↔host   uses 172.16.1.0/24 (host=.1,   router=.2)",
		"    server↔router uses 172.16.2.0/24 (server=.2, router=.1)",
		"    client↔router uses 172.16.3.0/24 (client=.2, router=.1)",
		"Implementation is a composition of `npte netns create`, `npte netns "+
			"connect`, `npte netns assign-addr`, `npte netns add-route`, and "+
			"`npte gateway create`. For non-default names, subnets, or shapes, "+
			"call those primitives directly.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<ext-iface>"
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

	extIface := fset.Args()[0]
	if err := validate.IfaceName(extIface); err != nil {
		logx.Error("npte star create: %s", err)
		env.Exit(2)
		return nil
	}

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

	logx.Details("npte: compose star topology (client/router/server) with gateway on %q", extIface)

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

	runSelf(ctx, self, pass("gateway", "create", "router", "172.16.1.0/24", extIface)...)

	logx.Details("npte: star topology is up")
	return nil
}
