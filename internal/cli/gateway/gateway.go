// SPDX-License-Identifier: GPL-3.0-or-later

// Package gateway implements the gateway subcommand.
package gateway

import (
	"context"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

// Main is the main of the gateway subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	disp := vclip.NewDispatcherCommand("npte gateway", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Manage internet-gateway state for network namespaces. A gateway namespace "+
			"has an uplink veth to the host, a default route, and host-side NAT so "+
			"that traffic from a tagged subnet egresses through a chosen external "+
			"interface.",
		"This is a higher-order concern layered on top of `npte netns`: the namespace "+
			"itself must already exist. All host-side state installed for a gateway is "+
			"tagged \"npte:gw:<ns>\" so that teardown can undo it without re-deriving "+
			"arguments.",
	)
	disp.AddCommand("create", vclip.CommandFunc(createMain), "Turn an existing namespace into a gateway.")

	return disp.Main(ctx, args)
}
