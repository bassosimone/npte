// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"

	"github.com/bassosimone/deferexit"
	"github.com/bassosimone/npte/internal/buildcfg"
	"github.com/bassosimone/npte/internal/cli/container"
	"github.com/bassosimone/npte/internal/cli/doctor"
	"github.com/bassosimone/npte/internal/cli/gateway"
	"github.com/bassosimone/npte/internal/cli/gencerts"
	"github.com/bassosimone/npte/internal/cli/lab"
	"github.com/bassosimone/npte/internal/cli/mcp"
	"github.com/bassosimone/npte/internal/cli/netem"
	"github.com/bassosimone/npte/internal/cli/netns"
	"github.com/bassosimone/npte/internal/cli/sudoers"
	"github.com/bassosimone/npte/internal/cli/tutorial"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

func main() {
	// The default testable.Env routes exits through deferexit as panics;
	// recover here turns them back into a real os.Exit after deferred
	// cleanup. See the [deferexit] package doc for why we do not instead
	// write `os.Exit(deferexit.Run(realMain))`.
	defer deferexit.Recover(os.Exit)
	realMain()
}

func realMain() {
	env := testable.Env

	disp := vclip.NewDispatcherCommand("npte", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Network Performance Testing Environment (npte). Test network "+
			"client performance under realistic conditions using isolated "+
			"network namespaces, traffic shaping, and optional lightweight "+
			"containers.",
		"npte is a collection of small, composable primitives: create and "+
			"connect namespaces (`netns`), attach a host-NATed uplink "+
			"(`gateway`), shape a link with tc/netem (`netem`), and "+
			"optionally run commands inside a systemd-nspawn container "+
			"(`container`). The `lab` command wires a fixed "+
			"client-router-server topology for the common case.",
		"Run `npte tutorial` for a walkthrough.",
	)
	disp.AddVersionHandlers(buildcfg.Version)
	disp.AddCommand("container", vclip.CommandFunc(container.Main), "Manage lightweight containers.")
	disp.AddCommand("doctor", vclip.CommandFunc(doctor.Main), "Check for required external commands.")
	disp.AddCommand("gateway", vclip.CommandFunc(gateway.Main), "Manage namespaces as internet gateways.")
	disp.AddCommand("gencerts", vclip.CommandFunc(gencerts.Main), "Generate self-signed TLS certificates for testing.")
	disp.AddCommand("lab", vclip.CommandFunc(lab.Main), "Compose a fixed three-node client/router/server lab.")
	disp.AddCommand("mcp", vclip.CommandFunc(mcp.Main), "Serve npte over MCP for agents (experimental).")
	disp.AddCommand("netem", vclip.CommandFunc(netem.Main), "Apply or clear traffic shaping.")
	disp.AddCommand("netns", vclip.CommandFunc(netns.Main), "Manage network namespaces.")
	disp.AddCommand("sudoers", vclip.CommandFunc(sudoers.Main), "Print a sudoers snippet for the invoking user.")
	disp.AddCommand("tutorial", vclip.CommandFunc(tutorial.Main), "Show the npte tutorials.")

	root := vclip.NewRootCommand(disp)
	root.LogFatalOnError0 = env.LogFatalOnError0
	root.Main(context.Background(), env.Args[1:])
}
