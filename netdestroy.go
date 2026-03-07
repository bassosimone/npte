// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"net"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netDestroyMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte net destroy", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Tear down all network namespaces and iptables rules created by npte. "+
			"Destroys endpoints first, then the central router, and finally removes the state file. "+
			"Individual teardown steps tolerate errors to handle partial initialization.",
		"The <project> argument selects the project to destroy. "+
			"After destruction, 'npte net init' can be used to recreate the project.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<project>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]

	unlock := mustLockNetState(proj)
	defer unlock()

	// Load network state
	sp := statePath(proj)
	logDetails("npte: load network state from %s\n", sp)
	state, err := loadNetState(proj)
	if err != nil {
		logAlways("npte net destroy: cannot load network state: %s\n", err)
		env.Exit(1)
	}

	// Destroy all endpoints first
	for _, hs := range state.Hosts {
		logDetails("npte: destroy endpoint '%s'\n", hs.Name)
		routerNs := nsName(proj, "router")
		routerVeth := proj + "-" + hs.Name + "-r"
		run("ip netns exec %s ip link del %s", routerNs, routerVeth)
		run("ip netns del %s", nsName(proj, hs.Name))
	}

	// Destroy the router: remove iptables rules, veth, and namespace
	logDetails("npte: destroy router\n")
	_, ipNet, err := net.ParseCIDR(state.RouterSubnet)
	env.LogFatalOnError0(err)
	insideAddr := ipWithOffset(ipNet, 2)
	hostVeth := proj + "-router-h"

	logDetails("npte: remove host NAT and FORWARD rules\n")
	run("iptables -D FORWARD -o %s -j ACCEPT", hostVeth)
	run("iptables -D FORWARD -i %s -j ACCEPT", hostVeth)
	run("iptables -t nat -D POSTROUTING -s %s/32 -j MASQUERADE", insideAddr)

	logDetails("npte: remove host veth %s and router namespace\n", hostVeth)
	run("ip link del %s", hostVeth)
	run("ip netns del %s", nsName(proj, "router"))

	// Remove state file
	logDetails("npte: remove state file %s\n", sp)
	logDetails("+ rm -f %s\n", sp)
	env.Remove(sp)

	logDetails("npte: network destroyed\n")
	return nil
}
