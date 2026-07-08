// SPDX-License-Identifier: GPL-3.0-or-later

// Package netns implements the netns subcommand.
package netns

import (
	"context"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

// Main is the main of the netns subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	disp := vclip.NewDispatcherCommand("npte netns", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Manage Linux network namespaces as composable primitives. Each subcommand "+
			"performs one operation: create or destroy a namespace, wire two namespaces "+
			"with a veth pair, assign addresses to interfaces, add routes, or run a "+
			"command inside a namespace.",
		"Every verb except `create` operates only on a namespace previously created "+
			"by `npte netns create`, tracked via a marker file at "+
			"`/run/npte/netns/<name>`. `list` enumerates managed namespaces; `show` "+
			"dumps diagnostics for one. There is no `--foreign` escape hatch — the "+
			"allowlisted surface is bounded to namespaces npte itself created. For "+
			"foreign namespaces, use `ip netns ...` directly.",
		"Topologies are built imperatively by composing these primitives; the only "+
			"persisted state is the per-namespace ownership marker. Requires root.",
	)
	disp.AddCommand("create", vclip.CommandFunc(createMain), "Create a minimal namespace.")
	disp.AddCommand("destroy", vclip.CommandFunc(destroyMain), "Destroy a namespace created by `create`.")
	disp.AddCommand("connect", vclip.CommandFunc(connectMain), "Wire two namespaces with a veth pair.")
	disp.AddCommand("assign-addr", vclip.CommandFunc(assignAddrMain), "Assign an IP address to an interface inside a namespace.")
	disp.AddCommand("add-route", vclip.CommandFunc(addRouteMain), "Add a route inside a namespace.")
	disp.AddCommand("run", vclip.CommandFunc(runMain), "Run a command inside a namespace.")
	disp.AddCommand("list", vclip.CommandFunc(listMain), "List namespaces managed by npte.")
	disp.AddCommand("show", vclip.CommandFunc(showMain), "Show diagnostics for a namespace managed by npte.")

	return disp.Main(ctx, args)
}
