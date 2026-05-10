// SPDX-License-Identifier: GPL-3.0-or-later

// Package mcp implements the mcp subcommand.
package mcp

import (
	"context"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

// Main is the main of the mcp subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	disp := vclip.NewDispatcherCommand("npte mcp", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Expose npte's privileged primitives over the Model Context "+
			"Protocol (MCP), so an agent running in its own sandbox can "+
			"invoke npte without shelling out to sudo. Experimental.",
		"The MCP server is a trust bridge, not a sandbox: it runs "+
			"outside the agent's sandbox and relies on npte's own "+
			"privilege drop and per-command sandboxing to keep the "+
			"invoked operation safe.",
	)
	disp.AddCommand("serve", vclip.CommandFunc(serveMain), "Speak MCP over stdio.")

	return disp.Main(ctx, args)
}
