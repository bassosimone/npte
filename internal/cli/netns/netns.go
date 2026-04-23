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
			"with a veth pair, assign addresses to interfaces, add routes, or bless a "+
			"namespace as an internet gateway.",
		"Topologies are built imperatively by composing these primitives; there is no "+
			"persisted project state. Requires root.",
	)
	disp.AddCommand("create", vclip.CommandFunc(createMain), "Create a minimal namespace.")
	disp.AddCommand("connect", vclip.CommandFunc(connectMain), "Wire two namespaces with a veth pair.")
	disp.AddCommand("assign-addr", vclip.CommandFunc(assignAddrMain), "Assign an IP address to an interface inside a namespace.")

	return disp.Main(ctx, args)
}
