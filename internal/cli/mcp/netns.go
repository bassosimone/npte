// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// netnsListInput is the input schema for the `netns_list` MCP tool.
type netnsListInput struct{}

// NetnsList is the MCP handler for the `netns_list` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) NetnsList(ctx context.Context, req *mcp.CallToolRequest,
	input *netnsListInput) (*mcp.CallToolResult, *runOutput, error) {
	out, err := sm.runProc(ctx, defaultWaitTimeout, []string{"netns", "list"})
	return nil, out, err
}

// netnsShowInput is the input schema for the `netns_show` MCP tool.
type netnsShowInput struct {
	Netns    string   `json:"netns" jsonschema:"Name of the npte-managed network namespace to inspect."`
	Sections []string `json:"sections,omitempty" jsonschema:"Restrict the dump to these section names (omit or empty to emit all). Valid names: link, addr, route, route6, qdisc, neigh, sockets, pids. Unknown names are silently ignored."`
}

// NetnsShow is the MCP handler for the `netns_show` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) NetnsShow(ctx context.Context, req *mcp.CallToolRequest,
	input *netnsShowInput) (*mcp.CallToolResult, *runOutput, error) {
	args := []string{"netns", "show"}
	for _, s := range input.Sections {
		args = append(args, "--section", s)
	}
	args = append(args, "--", input.Netns)
	out, err := sm.runProc(ctx, defaultWaitTimeout, args)
	return nil, out, err
}

// runCommandInput is the input schema for the `run_command` MCP tool.
//
// Note the deliberate absence of any `--sandbox` knob: the MCP server
// always invokes `npte netns run` with `--sandbox`, and the argv
// composition in [sessionManager.RunCommand] guards every user-supplied
// positional with a GNU `--` separator so that no field on this struct
// can be parsed as a flag.
type runCommandInput struct {
	Netns   string            `json:"netns" jsonschema:"Name of the npte-managed network namespace to enter."`
	Env     map[string]string `json:"env,omitempty" jsonschema:"Environment variables to set in the child, as a {name: value} map. Each entry becomes one '-e KEY=VALUE' flag. Keys must be valid environment-variable names; values are passed verbatim."`
	Argv    []string          `json:"argv" jsonschema:"Command and arguments to run inside the namespace. The first element is the program name (resolved via PATH inside the sandbox); the rest are its arguments. Must be non-empty."`
	Timeout string            `json:"timeout" jsonschema:"Maximum time to wait for the command to finish, as a Go duration string (e.g., \"30s\", \"2m\", \"1h\"). The tool returns as soon as the command terminates or this duration elapses, whichever comes first."`
}

// startCommandInput is the input schema for the `start_command` MCP tool.
//
// Note the deliberate absence of any `--sandbox` knob: the MCP server
// always invokes `npte netns run` with `--sandbox`, and the argv
// composition in [sessionManager.StartCommand] guards every user-supplied
// positional with a GNU `--` separator so that no field on this struct
// can be parsed as a flag.
type startCommandInput struct {
	Netns string            `json:"netns" jsonschema:"Name of the npte-managed network namespace to enter."`
	Env   map[string]string `json:"env,omitempty" jsonschema:"Environment variables to set in the child, as a {name: value} map. Each entry becomes one '-e KEY=VALUE' flag. Keys must be valid environment-variable names; values are passed verbatim."`
	Argv  []string          `json:"argv" jsonschema:"Command and arguments to run inside the namespace. The first element is the program name (resolved via PATH inside the sandbox); the rest are its arguments. Must be non-empty."`
}

// commandArgs builds the `npte netns run --sandbox ...` argv tail
// from the netns, env, and argv fields shared by [runCommandInput]
// and [startCommandInput].
func commandArgs(netns string, env map[string]string, argv []string) []string {
	args := []string{"netns", "run", "--sandbox"}
	for _, k := range slices.Sorted(maps.Keys(env)) {
		args = append(args, "-e", k+"="+env[k])
	}
	args = append(args, "--", netns)
	args = append(args, argv...)
	return args
}

// RunCommand is the MCP handler for the `run_command` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
//
// --sandbox is injected unconditionally, and the `--` separator before
// input.Netns terminates flag parsing so that neither the netns name
// nor any element of input.Argv can be interpreted as a flag — in
// particular, none of them can flip --sandbox=false. DisablePermute on
// the npte side is defense-in-depth; the `--` here is the structural
// guarantee.
func (sm *sessionManager) RunCommand(ctx context.Context, req *mcp.CallToolRequest,
	input *runCommandInput) (*mcp.CallToolResult, *runOutput, error) {
	timeout, err := time.ParseDuration(input.Timeout)
	if err != nil {
		return nil, nil, err
	}
	out, err := sm.runProc(ctx, timeout, commandArgs(input.Netns, input.Env, input.Argv))
	return nil, out, err
}

// StartCommand is the MCP handler for the `start_command` tool. See
// the tool's registration in [serveMain] for the agent-facing description.
//
// Same sandbox and `--` guarantees as [RunCommand]; the only difference
// is that the process is started in the background and requires explicit
// wait/kill.
func (sm *sessionManager) StartCommand(ctx context.Context, req *mcp.CallToolRequest,
	input *startCommandInput) (*mcp.CallToolResult, *startOutput, error) {
	proc, err := sm.startProc(commandArgs(input.Netns, input.Env, input.Argv))
	if err != nil {
		return nil, nil, err
	}
	out := &startOutput{
		ProcDir: proc.procDir,
		ProcID:  proc.procID,
	}
	return nil, out, nil
}
