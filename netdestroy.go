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
		"This command must be run as root (e.g., via sudo).",
	)
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	runtimex.PanicOnError0(fset.Parse(args))

	unlock := mustLockNetState()
	defer unlock()

	// Load network state
	logDetails("npte: load network state from %s\n", netStatePath)
	state, err := loadNetState()
	if err != nil {
		logAlways("npte net destroy: cannot load network state: %s\n", err)
		env.Exit(1)
	}

	pfx := state.Prefix

	// Destroy all endpoints first
	for _, hs := range state.Hosts {
		logDetails("npte: destroy endpoint '%s'\n", hs.Name)
		routerNs := nsName(pfx, "router")
		routerVeth := pfx + "-" + hs.Name + "-r"
		run("ip netns exec %s ip link del %s", routerNs, routerVeth)
		run("ip netns del %s", nsName(pfx, hs.Name))
	}

	// Destroy the router: remove iptables rules, veth, and namespace
	logDetails("npte: destroy router\n")
	_, ipNet, err := net.ParseCIDR(state.RouterSubnet)
	env.LogFatalOnError0(err)
	insideAddr := ipWithOffset(ipNet, 2)
	hostVeth := pfx + "-router-h"

	logDetails("npte: remove host NAT and FORWARD rules\n")
	run("iptables -D FORWARD -o %s -j ACCEPT", hostVeth)
	run("iptables -D FORWARD -i %s -j ACCEPT", hostVeth)
	run("iptables -t nat -D POSTROUTING -s %s/32 -j MASQUERADE", insideAddr)

	logDetails("npte: remove host veth %s and router namespace\n", hostVeth)
	run("ip link del %s", hostVeth)
	run("ip netns del %s", nsName(pfx, "router"))

	// Remove state file
	logDetails("npte: remove state file %s\n", netStatePath)
	logDetails("+ rm -f %s\n", netStatePath)
	env.Remove(netStatePath)

	logDetails("npte: network destroyed\n")
	return nil
}
