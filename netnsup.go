// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"net"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netnsUpMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte netns up", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Bring up the network namespaces described in the project configuration. "+
			"Creates the central router namespace with host NAT, then creates "+
			"all endpoint namespaces with their veth pairs and routes.",
		"The <project> argument selects the project whose network to bring up.",
		"Run this command after a reboot to recreate the network from the saved configuration. "+
			"If the network is already up, run 'npte netns down' first.",
		"This command must be run as root (e.g., via sudo).",
	)
	usage.PositionalArgumentsUsage = "<project>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]

	unlock := mustLockNetnsConfig(proj)
	defer unlock()

	// Load config
	logDetails("npte: load config from %s\n", configPath(proj))
	cfg := mustLoadNetnsConfig(proj)

	// Create the router namespace
	_, routerNet, err := net.ParseCIDR(routerSubnet)
	env.LogFatalOnError0(err)

	routerNs := nsName(proj, "router")
	hostVeth := proj + "-router-h"
	insideVeth := proj + "-router-i"
	hostAddr := ipWithOffset(routerNet, 1)
	insideAddr := ipWithOffset(routerNet, 2)
	ones, _ := routerNet.Mask.Size()
	cidr := fmt.Sprintf("%d", ones)

	logDetails("npte: create router namespace: %s\n", routerNs)
	mustRun("ip netns add %s", routerNs)
	mustRun("ip netns exec %s ip link set lo up", routerNs)
	mustRun("ip netns exec %s sysctl -w net.ipv4.ip_forward=1", routerNs)

	logDetails("npte: create veth pair %s <-> %s\n", hostVeth, insideVeth)
	mustRun("ip link add %s type veth peer name %s", hostVeth, insideVeth)
	mustRun("ip link set %s netns %s", insideVeth, routerNs)

	logDetails("npte: configure host side (%s/%s on %s)\n", hostAddr, cidr, hostVeth)
	mustRun("ip addr add %s/%s dev %s", hostAddr, cidr, hostVeth)
	mustRun("ip link set %s up", hostVeth)

	logDetails("npte: configure router side (%s/%s on %s)\n", insideAddr, cidr, insideVeth)
	mustRun("ip netns exec %s ip addr add %s/%s dev %s", routerNs, insideAddr, cidr, insideVeth)
	mustRun("ip netns exec %s ip link set %s up", routerNs, insideVeth)
	mustRun("ip netns exec %s ip route add default via %s", routerNs, hostAddr)

	logDetails("npte: SNAT endpoint traffic to %s inside the router\n", insideAddr)
	mustRun("ip netns exec %s iptables -t nat -A POSTROUTING -o %s -j SNAT --to-source %s",
		routerNs, insideVeth, insideAddr)

	logDetails("npte: enable host NAT and FORWARD rules for router traffic\n")
	mustRun("sysctl -w net.ipv4.ip_forward=1")
	mustRun("iptables -t nat -A POSTROUTING -s %s/32 -j MASQUERADE", insideAddr)
	mustRun("iptables -I FORWARD -s %s/32 -j ACCEPT", insideAddr)
	mustRun("iptables -I FORWARD -d %s/32 -j ACCEPT", insideAddr)

	// Create all endpoint namespaces
	for _, hs := range cfg.Hosts {
		logDetails("npte: create endpoint %q\n", hs.Name)

		_, ipNet, err := net.ParseCIDR(hs.Subnet)
		env.LogFatalOnError0(err)

		ns := nsName(proj, hs.Name)
		endpointVeth := proj + "-" + hs.Name + "-s"
		routerVeth := proj + "-" + hs.Name + "-r"
		routerAddr := ipWithOffset(ipNet, 1)
		endpointAddr := ipWithOffset(ipNet, 2)
		ones, _ := ipNet.Mask.Size()
		cidr := fmt.Sprintf("%d", ones)

		mustRun("ip netns add %s", ns)
		mustRun("ip netns exec %s ip link set lo up", ns)

		mustRun("ip link add %s type veth peer name %s", endpointVeth, routerVeth)
		mustRun("ip link set %s netns %s", endpointVeth, ns)
		mustRun("ip link set %s netns %s", routerVeth, routerNs)

		mustRun("ip netns exec %s ip addr add %s/%s dev %s", ns, endpointAddr, cidr, endpointVeth)
		mustRun("ip netns exec %s ip link set %s up", ns, endpointVeth)
		mustRun("ip netns exec %s ip route add default via %s", ns, routerAddr)

		mustRun("ip netns exec %s ip addr add %s/%s dev %s", routerNs, routerAddr, cidr, routerVeth)
		mustRun("ip netns exec %s ip link set %s up", routerNs, routerVeth)

		mustRun("ip netns exec %s sysctl -w net.ipv4.tcp_rmem='4096 131072 33554432'", ns)
		mustRun("ip netns exec %s sysctl -w net.ipv4.tcp_wmem='4096 131072 33554432'", ns)

		logDetails("npte: created %q with address %s\n", hs.Name, endpointAddr)
	}

	logDetails("npte: network is up\n")
	return nil
}
