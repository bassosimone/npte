// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"net"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netnsShowMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte netns show", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Show the network topology for a project. "+
			"Prints one line per interface with its namespace, address, and veth name. "+
			"The output is grep-friendly and suitable for scripting.",
		"The <project> argument selects the project whose topology to display.",
	)
	usage.PositionalArgumentsUsage = "<project>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]

	cfg := mustLoadNetnsConfig(proj)

	// Print endpoint rows
	for _, hs := range cfg.Hosts {
		_, ipNet, err := net.ParseCIDR(hs.Subnet)
		env.LogFatalOnError0(err)
		ones, _ := ipNet.Mask.Size()

		endpointAddr := ipWithOffset(ipNet, 2)
		routerAddr := ipWithOffset(ipNet, 1)

		endpointVeth := proj + "-" + hs.Name + "-s"
		routerVeth := proj + "-" + hs.Name + "-r"

		fmt.Fprintf(env.Stdout, "project %s netns %s addr %s mask %d veth %s\n",
			proj, nsName(proj, hs.Name), endpointAddr, ones, endpointVeth)
		fmt.Fprintf(env.Stdout, "project %s netns %s addr %s mask %d veth %s\n",
			proj, nsName(proj, "router"), routerAddr, ones, routerVeth)
	}

	// Print router host-facing row
	_, routerNet, err := net.ParseCIDR(routerSubnet)
	env.LogFatalOnError0(err)
	ones, _ := routerNet.Mask.Size()

	insideAddr := ipWithOffset(routerNet, 2)
	hostAddr := ipWithOffset(routerNet, 1)

	fmt.Fprintf(env.Stdout, "project %s netns %s addr %s mask %d veth %s\n",
		proj, nsName(proj, "router"), insideAddr, ones, proj+"-router-i")
	fmt.Fprintf(env.Stdout, "project %s netns %s addr %s mask %d veth %s\n",
		proj, "default", hostAddr, ones, proj+"-router-h")

	return nil
}
