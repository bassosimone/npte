// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"net"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netDownMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte net down", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Tear down all network namespaces and iptables rules for a project. "+
			"Destroys endpoints first, then the central router. "+
			"The configuration file is preserved so the network can be brought up again.",
		"The <project> argument selects the project whose network to tear down.",
		"Individual teardown steps tolerate errors to handle partial state.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<project>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]

	unlock := mustLockConfig(proj)
	defer unlock()

	// Load config
	logDetails("npte: load config from %s\n", configPath(proj))
	cfg := mustLoadConfig(proj)

	// Destroy all endpoints first
	for _, hs := range cfg.Hosts {
		logDetails("npte: destroy endpoint %q\n", hs.Name)
		routerNs := nsName(proj, "router")
		routerVeth := proj + "-" + hs.Name + "-r"
		run("ip netns exec %s ip link del %s", routerNs, routerVeth)
		run("ip netns del %s", nsName(proj, hs.Name))
	}

	// Destroy the router
	logDetails("npte: destroy router\n")
	_, routerNet, err := net.ParseCIDR(routerSubnet)
	env.LogFatalOnError0(err)
	insideAddr := ipWithOffset(routerNet, 2)
	hostVeth := proj + "-router-h"

	logDetails("npte: remove host NAT and FORWARD rules\n")
	run("iptables -D FORWARD -o %s -j ACCEPT", hostVeth)
	run("iptables -D FORWARD -i %s -j ACCEPT", hostVeth)
	run("iptables -t nat -D POSTROUTING -s %s/32 -j MASQUERADE", insideAddr)

	logDetails("npte: remove host veth %s and router namespace\n", hostVeth)
	run("ip link del %s", hostVeth)
	run("ip netns del %s", nsName(proj, "router"))

	logDetails("npte: network is down\n")
	return nil
}
