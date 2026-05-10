// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"maps"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// netnsListInput is the input schema for the `start_netns_list` MCP tool.
type netnsListInput struct{}

// NetnsList is the MCP handler for the `start_netns_list` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) NetnsList(ctx context.Context, req *mcp.CallToolRequest,
	input *netnsListInput) (*mcp.CallToolResult, *startOutput, error) {
	out, err := sm.startProc([]string{"netns", "list"})
	return nil, out, err
}

// netnsShowInput is the input schema for the `start_netns_show` MCP tool.
type netnsShowInput struct {
	Netns    string   `json:"netns" jsonschema:"Name of the npte-managed network namespace to inspect."`
	Sections []string `json:"sections,omitempty" jsonschema:"Restrict the dump to these section names (omit or empty to emit all). Valid names: link, addr, route, route6, qdisc, neigh, sockets, pids. Unknown names are silently ignored."`
}

// NetnsShow is the MCP handler for the `start_netns_show` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) NetnsShow(ctx context.Context, req *mcp.CallToolRequest,
	input *netnsShowInput) (*mcp.CallToolResult, *startOutput, error) {
	args := []string{"netns", "show"}
	for _, s := range input.Sections {
		args = append(args, "--section", s)
	}
	args = append(args, input.Netns)
	out, err := sm.startProc(args)
	return nil, out, err
}

// netnsRunInput is the input schema for the `start_netns_run` MCP tool.
//
// Note the deliberate absence of any `--sandbox` knob: the MCP server
// always invokes `npte netns run` with `--sandbox`, and there is no
// flag-bag field through which the agent could disable it.
type netnsRunInput struct {
	Netns string            `json:"netns" jsonschema:"Name of the npte-managed network namespace to enter."`
	Env   map[string]string `json:"env,omitempty" jsonschema:"Environment variables to set in the child, as a {name: value} map. Each entry becomes one '-e KEY=VALUE' flag. Keys must be valid environment-variable names; values are passed verbatim."`
	Argv  []string          `json:"argv" jsonschema:"Command and arguments to run inside the namespace. The first element is the program name (resolved via PATH inside the sandbox); the rest are its arguments. Must be non-empty."`
}

// NetnsRun is the MCP handler for the `start_netns_run` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
//
// --sandbox is injected unconditionally; npte netns run has DisablePermute,
// so any flag-like token in input.Argv lands as a positional and cannot
// retroactively flip --sandbox=false.
func (sm *sessionManager) NetnsRun(ctx context.Context, req *mcp.CallToolRequest,
	input *netnsRunInput) (*mcp.CallToolResult, *startOutput, error) {
	args := []string{"netns", "run", "--sandbox"}
	for _, k := range slices.Sorted(maps.Keys(input.Env)) {
		args = append(args, "-e", k+"="+input.Env[k])
	}
	args = append(args, input.Netns)
	args = append(args, input.Argv...)
	out, err := sm.startProc(args)
	return nil, out, err
}
