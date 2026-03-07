// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"net"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netInitMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte net init", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Create the central router namespace with a host-facing veth pair for internet access. "+
			"Configures NAT masquerade and iptables FORWARD rules on the host so that "+
			"endpoints created later can reach the internet through the router.",
		"This command must be run as root (e.g., via sudo).",
	)
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	runtimex.PanicOnError0(fset.Parse(args))

	unlock := mustLockNetState()
	defer unlock()

	pfx := mustLoadPrefix()
	if _, err := env.Stat(netStatePath); err == nil {
		logAlways("npte net init: network already initialized\n")
		logAlways("npte net init: run `npte net destroy' first\n")
		env.Exit(1)
	}
	logDetails("npte: creating router namespace with prefix: %s\n", pfx)

	// The router↔host link uses a fixed subnet: 10.0.0.0/24
	routerSubnet := "10.0.0.0/24"
	_, ipNet, err := net.ParseCIDR(routerSubnet)
	env.LogFatalOnError0(err)

	routerNs := nsName(pfx, "router")
	hostVeth := pfx + "-router-h"
	insideVeth := pfx + "-router-i"
	hostAddr := ipWithOffset(ipNet, 1)
	insideAddr := ipWithOffset(ipNet, 2)
	ones, _ := ipNet.Mask.Size()
	cidr := fmt.Sprintf("%d", ones)

	logDetails("npte: router <-> host subnet is %s\n", routerSubnet)

	// Save state early so that `npte net destroy` can clean up partial initialization
	logDetails("npte: save initial state to %s\n", netStatePath)
	env.LogFatalOnError0(saveNetState(&netState{
		Prefix:          pfx,
		RouterSubnet:    routerSubnet,
		NextSubnetIndex: 1,
		Hosts:           make(map[string]*hostState),
	}))

	// Create the router namespace with IP forwarding enabled
	logDetails("npte: create router namespace: %s\n", routerNs)
	mustRun("ip netns add %s", routerNs)
	mustRun("ip netns exec %s ip link set lo up", routerNs)
	mustRun("ip netns exec %s sysctl -w net.ipv4.ip_forward=1", routerNs)

	// Create host↔router veth pair for internet access
	logDetails("npte: create veth pair %s <-> %s\n", hostVeth, insideVeth)
	mustRun("ip link add %s type veth peer name %s", hostVeth, insideVeth)
	mustRun("ip link set %s netns %s", insideVeth, routerNs)

	// Configure host side of the veth
	logDetails("npte: configure host side (%s/%s on %s)\n", hostAddr, cidr, hostVeth)
	mustRun("ip addr add %s/%s dev %s", hostAddr, cidr, hostVeth)
	mustRun("ip link set %s up", hostVeth)

	// Configure router side of the veth with default route to host
	logDetails("npte: configure router side (%s/%s on %s)\n", insideAddr, cidr, insideVeth)
	mustRun("ip netns exec %s ip addr add %s/%s dev %s", routerNs, insideAddr, cidr, insideVeth)
	mustRun("ip netns exec %s ip link set %s up", routerNs, insideVeth)
	mustRun("ip netns exec %s ip route add default via %s", routerNs, hostAddr)

	// Enable NAT on the host so the router can reach the internet
	logDetails("npte: enable host NAT and FORWARD rules for router traffic\n")
	mustRun("sysctl -w net.ipv4.ip_forward=1")
	mustRun("iptables -t nat -A POSTROUTING -s %s/32 -j MASQUERADE", insideAddr)
	mustRun("iptables -I FORWARD -i %s -j ACCEPT", hostVeth)
	mustRun("iptables -I FORWARD -o %s -j ACCEPT", hostVeth)

	logDetails("npte: router initialized\n")
	return nil
}
