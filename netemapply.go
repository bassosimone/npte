// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func netemApplyMain(ctx context.Context, args []string) error {
	var rtt, download, upload string

	fset := vflag.NewFlagSet("npte netem apply", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Apply traffic shaping to a client's access link. "+
			"Clears any existing netem rules on the client's interfaces, "+
			"then applies the specified delay and rate in both directions.",
		"The <project> and <client> arguments select which endpoint to shape. "+
			"The RTT is split equally between the upload and download paths.",
		"This command requires root privileges (e.g., via sudo). "+
			"See 'npte tutorial netem' for details and advanced usage.",
	)
	usage.PositionalArgumentsUsage = "<project> <client>"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&rtt, 0, "rtt", "Round-trip time (e.g., 60ms). Split equally between directions.")
	fset.StringVar(&download, 0, "download", "Download rate (e.g., 30mbit).")
	fset.StringVar(&upload, 0, "upload", "Upload rate (e.g., 10mbit).")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = 2
	runtimex.PanicOnError0(fset.Parse(args))

	proj := fset.Args()[0]
	client := fset.Args()[1]

	if err := validateProject(proj); err != nil {
		logx.Error("npte netem apply: %s", err)
		env.Exit(2)
	}
	if err := validateEndpointName(proj, client); err != nil {
		logx.Error("npte netem apply: %s", err)
		env.Exit(2)
	}
	if rtt == "" {
		logx.Error("npte netem apply: --rtt is required")
		env.Exit(2)
	}

	oneWayDelay, err := halfRTT(rtt)
	if err != nil {
		logx.Error("npte netem apply: %s", err)
		env.Exit(2)
	}

	// Load config to verify the client endpoint exists
	unlock := mustLockNetnsConfig(proj)
	defer unlock()
	cfg := mustLoadNetnsConfig(proj)
	if _, ok := cfg.Hosts[client]; !ok {
		logx.Error("npte netem apply: endpoint %q not found in project %q", client, proj)
		env.Exit(1)
	}

	routerNs := nsName(proj, "router")
	clientNs := nsName(proj, client)
	dlIface := proj + "-" + client + "-r"
	ulIface := proj + "-" + client + "-s"

	// Clear existing rules (ignore errors — may not have any)
	logx.Details("npte: clear existing netem rules")
	runCmd(ctx, "ip netns exec %s tc qdisc del dev %s root", routerNs, dlIface)
	runCmd(ctx, "ip netns exec %s tc qdisc del dev %s root", clientNs, ulIface)

	// Build the netem arguments for each direction
	dlNetem := "delay " + oneWayDelay
	if download != "" {
		dlNetem += " rate " + download
	}
	ulNetem := "delay " + oneWayDelay
	if upload != "" {
		ulNetem += " rate " + upload
	}

	// Apply new rules
	logx.Details("npte: apply download shaping on %s (%s)", dlIface, dlNetem)
	mustRunCmd(ctx, "ip netns exec %s tc qdisc add dev %s root netem %s", routerNs, dlIface, dlNetem)

	logx.Details("npte: apply upload shaping on %s (%s)", ulIface, ulNetem)
	mustRunCmd(ctx, "ip netns exec %s tc qdisc add dev %s root netem %s", clientNs, ulIface, ulNetem)

	logx.Details("npte: shaping applied (RTT %s, download %s, upload %s)", rtt, download, upload)
	return nil
}
