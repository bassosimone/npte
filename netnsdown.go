// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netnsDownMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte netns down", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Tear down all network namespaces and iptables rules for a project. "+
			"Destroys endpoints first, then the central router. "+
			"The configuration file is preserved so the network can be brought up again.",
		"The <project> argument selects the project whose network namespaces to tear down. "+
			"Individual teardown steps tolerate errors to handle partial/inconsistent state gracefully.",
		"This command requires root privileges (e.g., via sudo). "+
			"See 'npte tutorial namespaces' for details.",
	)
	usage.PositionalArgumentsUsage = "<project>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	if err := validateProject(proj); err != nil {
		logError("npte netns down: %s", err)
		env.Exit(2)
	}

	unlock := mustLockNetnsConfig(proj)
	defer unlock()

	// Load config
	logDetails("npte: load config from %s", configPath(proj))
	cfg := mustLoadNetnsConfig(proj)

	// Destroy all endpoints first
	for _, hs := range cfg.Hosts {
		logDetails("npte: destroy endpoint %q", hs.Name)
		routerNs := nsName(proj, "router")
		routerVeth := proj + "-" + hs.Name + "-r"
		runCmd(ctx, "ip netns exec %s ip link del %s", routerNs, routerVeth)
		runCmd(ctx, "ip netns del %s", nsName(proj, hs.Name))
	}

	// Destroy the router
	logDetails("npte: destroy router")
	routerNet := cfg.mustSubnet(0)
	insideAddr := ipWithOffset(routerNet, 2)
	hostVeth := proj + "-router-h"

	logDetails("npte: remove host NAT and FORWARD rules")
	runCmd(ctx, "iptables -D FORWARD -s %s/32 -j ACCEPT", insideAddr)
	runCmd(ctx, "iptables -D FORWARD -d %s/32 -j ACCEPT", insideAddr)
	runCmd(ctx, "iptables -t nat -D POSTROUTING -s %s/32 -j MASQUERADE", insideAddr)

	logDetails("npte: remove host veth %s and router namespace", hostVeth)
	runCmd(ctx, "ip link del %s", hostVeth)
	runCmd(ctx, "ip netns del %s", nsName(proj, "router"))

	logDetails("npte: network is down")
	return nil
}
