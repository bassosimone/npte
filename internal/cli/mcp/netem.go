// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// netemApplyInput is the input schema for the `start_netem_apply` MCP tool.
//
// Every npte `netem apply` flag is exposed as a typed field. Values are
// passed verbatim to npte, which validates them against the tc-netem
// grammar; surface those errors to the agent via stderr.txt.
type netemApplyInput struct {
	Netns         string `json:"netns" jsonschema:"Name of the npte-managed network namespace containing the interface to shape."`
	Iface         string `json:"iface" jsonschema:"Name of the interface inside <netns> to install the netem qdisc on."`
	Delay         string `json:"delay,omitempty" jsonschema:"Pass-through to netem 'delay' (e.g., \"10ms\", \"10ms 2ms distribution paretonormal\")."`
	Loss          string `json:"loss,omitempty" jsonschema:"Pass-through to netem 'loss' (e.g., \"1%\", \"gemodel 0.1 0.05 0.9 0.95\")."`
	Limit         string `json:"limit,omitempty" jsonschema:"Pass-through to netem 'limit' in packets (e.g., \"1000\")."`
	Rate          string `json:"rate,omitempty" jsonschema:"Pass-through to netem 'rate' (e.g., \"10mbit\", \"10mbit 1000 500\")."`
	Slot          string `json:"slot,omitempty" jsonschema:"Pass-through to netem 'slot' (e.g., \"5ms 10ms packets 64\")."`
	Child         string `json:"child,omitempty" jsonschema:"Child qdisc kind attached at parent 1: (one of: cake, fq_codel, pie, sfq, codel; see npte's validate.AllowedChildQdiscs for the authoritative list)."`
	CakeBandwidth string `json:"cakeBandwidth,omitempty" jsonschema:"Bandwidth for the 'cake' child qdisc (e.g., \"30mbit\"). Ignored unless child == \"cake\"."`
}

// NetemApply is the MCP handler for the `start_netem_apply` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) NetemApply(ctx context.Context, req *mcp.CallToolRequest,
	input *netemApplyInput) (*mcp.CallToolResult, *startOutput, error) {
	args := []string{"netem", "apply"}
	for _, kv := range []struct {
		key, value string
	}{
		{"--delay", input.Delay},
		{"--loss", input.Loss},
		{"--limit", input.Limit},
		{"--rate", input.Rate},
		{"--slot", input.Slot},
		{"--child", input.Child},
		{"--cake-bandwidth", input.CakeBandwidth},
	} {
		if kv.value != "" {
			args = append(args, kv.key, kv.value)
		}
	}
	args = append(args, "--", input.Netns, input.Iface)
	out, err := sm.startProc(args)
	return nil, out, err
}

// netemClearInput is the input schema for the `start_netem_clear` MCP tool.
type netemClearInput struct {
	Netns string `json:"netns" jsonschema:"Name of the npte-managed network namespace containing the interface to unshape."`
	Iface string `json:"iface" jsonschema:"Name of the interface inside <netns> to remove the root qdisc from."`
}

// NetemClear is the MCP handler for the `start_netem_clear` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) NetemClear(ctx context.Context, req *mcp.CallToolRequest,
	input *netemClearInput) (*mcp.CallToolResult, *startOutput, error) {
	out, err := sm.startProc([]string{"netem", "clear", "--", input.Netns, input.Iface})
	return nil, out, err
}
