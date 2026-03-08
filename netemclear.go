// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netemClearMain(ctx context.Context, args []string) error {
	fset := vflag.NewFlagSet("npte netem clear", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Remove all netem traffic shaping rules from a client's access link.",
		"The <project> and <client> arguments select which endpoint to clear. "+
			"Tolerates errors if no rules are currently applied.",
		"This command requires root privileges (e.g., via sudo). "+
			"See 'npte tutorial netem' for details.",
	)
	usage.PositionalArgumentsUsage = "<project> <client>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = 2
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	client := fset.Args()[1]

	if err := validateProject(proj); err != nil {
		logError("npte netem clear: %s", err)
		env.Exit(2)
	}
	if err := validateEndpointName(proj, client); err != nil {
		logError("npte netem clear: %s", err)
		env.Exit(2)
	}

	// Load config to verify the client endpoint exists
	unlock := mustLockNetnsConfig(proj)
	defer unlock()
	cfg := mustLoadNetnsConfig(proj)
	if _, ok := cfg.Hosts[client]; !ok {
		logError("npte netem clear: endpoint %q not found in project %q", client, proj)
		env.Exit(1)
	}

	routerNs := nsName(proj, "router")
	clientNs := nsName(proj, client)
	dlIface := proj + "-" + client + "-r"
	ulIface := proj + "-" + client + "-s"

	logDetails("npte: clear netem rules on %s and %s", dlIface, ulIface)
	runCmd(ctx, "ip netns exec %s tc qdisc del dev %s root", routerNs, dlIface)
	runCmd(ctx, "ip netns exec %s tc qdisc del dev %s root", clientNs, ulIface)

	logDetails("npte: shaping cleared")
	return nil
}
