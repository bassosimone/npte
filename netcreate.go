// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"net"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netCreateMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte net create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Create a new network namespace connected to the central router. "+
			"Automatically allocates a /24 subnet, configures a veth pair between "+
			"the endpoint and the router namespace, and tunes TCP buffer sizes. "+
			"The new endpoint has internet access through the router.",
		"The <project> argument selects the project. "+
			"The <name> argument is the name of the network namespace to create.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<project> <name>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = 2
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	nameFlag := fset.Args()[1]

	unlock := mustLockNetState(proj)
	defer unlock()

	// Load network state
	logDetails("npte: load network state from %s\n", statePath(proj))
	state := mustLoadNetState(proj)

	if err := validateEndpointName(proj, nameFlag); err != nil {
		logAlways("npte net create: %s\n", err)
		env.Exit(2)
	}

	if _, exists := state.Hosts[nameFlag]; exists {
		logAlways("npte net create: host '%s' already exists\n", nameFlag)
		env.Exit(1)
	}

	// Auto-allocate subnet: 10.0.<index>.0/24
	subnet := fmt.Sprintf("10.0.%d.0/24", state.NextSubnetIndex)
	_, ipNet, err := net.ParseCIDR(subnet)
	env.LogFatalOnError0(err)

	ns := nsName(proj, nameFlag)
	routerNs := nsName(proj, "router")

	endpointVeth := proj + "-" + nameFlag + "-s"
	routerVeth := proj + "-" + nameFlag + "-r"

	// Convention: .1 is router side, .2 is endpoint side
	routerAddr := ipWithOffset(ipNet, 1)
	endpointAddr := ipWithOffset(ipNet, 2)
	ones, _ := ipNet.Mask.Size()
	cidr := fmt.Sprintf("%d", ones)

	logDetails("npte: allocate subnet %s for endpoint '%s'\n", subnet, nameFlag)

	// Save state early so that `npte net destroy` can clean up partial initialization
	logDetails("npte: save updated state to %s\n", statePath(proj))
	state.Hosts[nameFlag] = &hostState{
		Name:   nameFlag,
		Subnet: subnet,
	}
	state.NextSubnetIndex++
	env.LogFatalOnError0(saveNetState(proj, state))

	// Create the endpoint namespace
	logDetails("npte: create network namespace '%s'\n", ns)
	mustRun("ip netns add %s", ns)
	mustRun("ip netns exec %s ip link set lo up", ns)

	// Create veth pair and move to respective namespaces
	logDetails("npte: create veth pair %s <-> %s\n", endpointVeth, routerVeth)
	mustRun("ip link add %s type veth peer name %s", endpointVeth, routerVeth)
	mustRun("ip link set %s netns %s", endpointVeth, ns)
	mustRun("ip link set %s netns %s", routerVeth, routerNs)

	// Configure endpoint side: address and default route via router
	logDetails("npte: configure endpoint side (%s/%s on %s)\n", endpointAddr, cidr, endpointVeth)
	mustRun("ip netns exec %s ip addr add %s/%s dev %s", ns, endpointAddr, cidr, endpointVeth)
	mustRun("ip netns exec %s ip link set %s up", ns, endpointVeth)
	mustRun("ip netns exec %s ip route add default via %s", ns, routerAddr)

	// Configure router side: address on the router's end of the veth
	logDetails("npte: configure router side (%s/%s on %s)\n", routerAddr, cidr, routerVeth)
	mustRun("ip netns exec %s ip addr add %s/%s dev %s", routerNs, routerAddr, cidr, routerVeth)
	mustRun("ip netns exec %s ip link set %s up", routerNs, routerVeth)

	// TCP buffer tuning for high bandwidth-delay product paths
	logDetails("npte: tune TCP buffer sizes inside '%s'\n", ns)
	mustRun("ip netns exec %s sysctl -w net.ipv4.tcp_rmem='4096 131072 33554432'", ns)
	mustRun("ip netns exec %s sysctl -w net.ipv4.tcp_wmem='4096 131072 33554432'", ns)

	logDetails("npte: created '%s' with address %s\n", nameFlag, endpointAddr)
	return nil
}
