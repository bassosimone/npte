// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netnsUpMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte netns up", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Bring up the network namespaces described in the given project configuration. "+
			"Creates the central router namespace with host NAT and internet access, then creates "+
			"all endpoint namespaces with their veth pairs and routes.",
		"The <project> argument selects the project whose network to bring up. "+
			"Run this command after a reboot to recreate the network. "+
			"Use 'npte netns status' to check for status. "+
			"If the network is already up, run 'npte netns down' to tear it down first.",
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
		logError("npte netns up: %s", err)
		env.Exit(2)
	}

	unlock := mustLockNetnsConfig(proj)
	defer unlock()

	// Load config
	logDetails("npte: load config from %s", configPath(proj))
	cfg := mustLoadNetnsConfig(proj)

	// Create the router namespace
	routerNet := cfg.mustSubnet(0)

	routerNs := nsName(proj, "router")
	hostVeth := proj + "-router-h"
	insideVeth := proj + "-router-i"
	hostAddr := ipWithOffset(routerNet, 1)
	insideAddr := ipWithOffset(routerNet, 2)
	ones, _ := routerNet.Mask.Size()
	cidr := fmt.Sprintf("%d", ones)

	logDetails("npte: create router namespace: %s", routerNs)
	mustRunCmd(ctx, "ip netns add %s", routerNs)
	mustRunCmd(ctx, "ip netns exec %s ip link set lo up", routerNs)
	mustRunCmd(ctx, "ip netns exec %s sysctl -w net.ipv4.ip_forward=1", routerNs)

	logDetails("npte: create veth pair %s <-> %s", hostVeth, insideVeth)
	mustRunCmd(ctx, "ip link add %s type veth peer name %s", hostVeth, insideVeth)
	mustRunCmd(ctx, "ip link set %s netns %s", insideVeth, routerNs)

	logDetails("npte: configure host side (%s/%s on %s)", hostAddr, cidr, hostVeth)
	mustRunCmd(ctx, "ip addr add %s/%s dev %s", hostAddr, cidr, hostVeth)
	mustRunCmd(ctx, "ip link set %s up", hostVeth)

	logDetails("npte: configure router side (%s/%s on %s)", insideAddr, cidr, insideVeth)
	mustRunCmd(ctx, "ip netns exec %s ip addr add %s/%s dev %s", routerNs, insideAddr, cidr, insideVeth)
	mustRunCmd(ctx, "ip netns exec %s ip link set %s up", routerNs, insideVeth)
	mustRunCmd(ctx, "ip netns exec %s ip route add default via %s", routerNs, hostAddr)

	logDetails("npte: SNAT endpoint traffic to %s inside the router", insideAddr)
	mustRunCmd(ctx, "ip netns exec %s iptables -t nat -A POSTROUTING -o %s -j SNAT --to-source %s",
		routerNs, insideVeth, insideAddr)

	logDetails("npte: enable host NAT and FORWARD rules for router traffic")
	mustRunCmd(ctx, "sysctl -w net.ipv4.ip_forward=1")
	mustRunCmd(ctx, "iptables -t nat -A POSTROUTING -s %s/32 -j MASQUERADE", insideAddr)
	mustRunCmd(ctx, "iptables -I FORWARD -s %s/32 -j ACCEPT", insideAddr)
	mustRunCmd(ctx, "iptables -I FORWARD -d %s/32 -j ACCEPT", insideAddr)

	// Create all endpoint namespaces
	for _, hs := range cfg.Hosts {
		logDetails("npte: create endpoint %q", hs.Name)

		ipNet := cfg.mustSubnet(hs.SubnetIndex)

		ns := nsName(proj, hs.Name)
		endpointVeth := proj + "-" + hs.Name + "-s"
		routerVeth := proj + "-" + hs.Name + "-r"
		routerAddr := ipWithOffset(ipNet, 1)
		endpointAddr := ipWithOffset(ipNet, 2)
		ones, _ := ipNet.Mask.Size()
		cidr := fmt.Sprintf("%d", ones)

		mustRunCmd(ctx, "ip netns add %s", ns)
		mustRunCmd(ctx, "ip netns exec %s ip link set lo up", ns)

		mustRunCmd(ctx, "ip link add %s type veth peer name %s", endpointVeth, routerVeth)
		mustRunCmd(ctx, "ip link set %s netns %s", endpointVeth, ns)
		mustRunCmd(ctx, "ip link set %s netns %s", routerVeth, routerNs)

		mustRunCmd(ctx, "ip netns exec %s ip addr add %s/%s dev %s", ns, endpointAddr, cidr, endpointVeth)
		mustRunCmd(ctx, "ip netns exec %s ip link set %s up", ns, endpointVeth)
		mustRunCmd(ctx, "ip netns exec %s ip route add default via %s", ns, routerAddr)

		mustRunCmd(ctx, "ip netns exec %s ip addr add %s/%s dev %s", routerNs, routerAddr, cidr, routerVeth)
		mustRunCmd(ctx, "ip netns exec %s ip link set %s up", routerNs, routerVeth)

		mustRunCmd(ctx, "ip netns exec %s sysctl -w net.ipv4.tcp_rmem='4096 131072 33554432'", ns)
		mustRunCmd(ctx, "ip netns exec %s sysctl -w net.ipv4.tcp_wmem='4096 131072 33554432'", ns)

		logDetails("npte: created %q with address %s", hs.Name, endpointAddr)
	}

	logDetails("npte: network is up")
	return nil
}
