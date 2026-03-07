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
		"Create a new network endpoint namespace connected to the central router. "+
			"Automatically allocates a /24 subnet and configures a veth pair between "+
			"the endpoint and the router namespace.",
		"The <name> argument is the name of the network namespace to create.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<name>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	nameFlag := fset.Args()[0]

	unlock := mustLockNetState()
	defer unlock()

	// Load network state
	fmt.Fprintf(env.Stderr, "npte: load network state from %s\n", netStatePath)
	state := mustLoadNetState()

	if err := validateEndpointName(state.Prefix, nameFlag); err != nil {
		fmt.Fprintf(env.Stderr, "npte net create: %s\n", err)
		env.Exit(2)
	}

	if _, exists := state.Hosts[nameFlag]; exists {
		fmt.Fprintf(env.Stderr, "npte net create: host '%s' already exists\n", nameFlag)
		env.Exit(1)
	}

	// Auto-allocate subnet: 10.0.<index>.0/24
	subnet := fmt.Sprintf("10.0.%d.0/24", state.NextSubnetIndex)
	_, ipNet, err := net.ParseCIDR(subnet)
	env.LogFatalOnError0(err)

	pfx := state.Prefix
	ns := nsName(pfx, nameFlag)
	routerNs := nsName(pfx, "router")

	endpointVeth := pfx + "-" + nameFlag + "-s"
	routerVeth := pfx + "-" + nameFlag + "-r"

	// Convention: .1 is router side, .2 is endpoint side
	routerAddr := ipWithOffset(ipNet, 1)
	endpointAddr := ipWithOffset(ipNet, 2)
	ones, _ := ipNet.Mask.Size()
	cidr := fmt.Sprintf("%d", ones)

	fmt.Fprintf(env.Stderr, "npte: allocate subnet %s for endpoint '%s'\n", subnet, nameFlag)

	// Save state early so that `npte net destroy` can clean up partial initialization
	fmt.Fprintf(env.Stderr, "npte: save updated state to %s\n", netStatePath)
	state.Hosts[nameFlag] = &hostState{
		Name:   nameFlag,
		Subnet: subnet,
	}
	state.NextSubnetIndex++
	env.LogFatalOnError0(saveNetState(state))

	// Create the endpoint namespace
	fmt.Fprintf(env.Stderr, "npte: create network namespace '%s'\n", ns)
	mustRun("ip netns add %s", ns)
	mustRun("ip netns exec %s ip link set lo up", ns)

	// Create veth pair and move to respective namespaces
	fmt.Fprintf(env.Stderr, "npte: create veth pair %s <-> %s\n", endpointVeth, routerVeth)
	mustRun("ip link add %s type veth peer name %s", endpointVeth, routerVeth)
	mustRun("ip link set %s netns %s", endpointVeth, ns)
	mustRun("ip link set %s netns %s", routerVeth, routerNs)

	// Configure endpoint side: address and default route via router
	fmt.Fprintf(env.Stderr, "npte: configure endpoint side (%s/%s on %s)\n", endpointAddr, cidr, endpointVeth)
	mustRun("ip netns exec %s ip addr add %s/%s dev %s", ns, endpointAddr, cidr, endpointVeth)
	mustRun("ip netns exec %s ip link set %s up", ns, endpointVeth)
	mustRun("ip netns exec %s ip route add default via %s", ns, routerAddr)

	// Configure router side: address on the router's end of the veth
	fmt.Fprintf(env.Stderr, "npte: configure router side (%s/%s on %s)\n", routerAddr, cidr, routerVeth)
	mustRun("ip netns exec %s ip addr add %s/%s dev %s", routerNs, routerAddr, cidr, routerVeth)
	mustRun("ip netns exec %s ip link set %s up", routerNs, routerVeth)

	// TCP buffer tuning for high bandwidth-delay product paths
	fmt.Fprintf(env.Stderr, "npte: tune TCP buffer sizes inside '%s'\n", ns)
	mustRun("ip netns exec %s sysctl -w net.ipv4.tcp_rmem='4096 131072 33554432'", ns)
	mustRun("ip netns exec %s sysctl -w net.ipv4.tcp_wmem='4096 131072 33554432'", ns)

	fmt.Fprintf(env.Stderr, "npte: created '%s' with address %s\n", nameFlag, endpointAddr)
	return nil
}
