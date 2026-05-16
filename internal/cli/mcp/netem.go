// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// validateNetemTarget enforces the MCP-layer allowlist for the netem
// tools: only the canonical hub of the canned lab (`router`) and its
// two veth endpoints (`if-client`, `if-server`) are accepted. See
// invariant #6 in CLAUDE.md.
func validateNetemTarget(netns, iface string) error {
	if netns != "router" {
		return fmt.Errorf(
			"netns must be \"router\" (got %q); the MCP only shapes "+
				"at the canonical hub of the canned lab. Shaping at "+
				"a leaf namespace (client, server) is rejected. See "+
				"server instructions for the lab shaping convention",
			netns,
		)
	}
	if iface != "if-client" && iface != "if-server" {
		return fmt.Errorf(
			"iface must be \"if-client\" (router->client egress) or "+
				"\"if-server\" (router->server egress); got %q",
			iface,
		)
	}
	return nil
}

// netemApplyInput is the input schema for the `netem_apply` MCP tool.
//
// Every npte `netem apply` flag is exposed as a typed field. Values are
// passed verbatim to npte, which validates them against the tc-netem
// grammar; surface those errors to the agent via stderr.txt.
type netemApplyInput struct {
	Netns         string `json:"netns" jsonschema:"Must be \"router\". The MCP only shapes at the canonical hub of the canned lab; shaping at a leaf namespace (client, server) is rejected. See server instructions for the lab shaping convention."`
	Iface         string `json:"iface" jsonschema:"Must be \"if-client\" (router->client egress) or \"if-server\" (router->server egress)."`
	Delay         string `json:"delay,omitempty" jsonschema:"Pass-through to netem 'delay' (e.g., \"10ms\", \"10ms 2ms distribution paretonormal\")."`
	Loss          string `json:"loss,omitempty" jsonschema:"Pass-through to netem 'loss' (e.g., \"1%\", \"gemodel 0.1 0.05 0.9 0.95\")."`
	Limit         string `json:"limit,omitempty" jsonschema:"Pass-through to netem 'limit' in packets (e.g., \"1000\")."`
	Rate          string `json:"rate,omitempty" jsonschema:"Pass-through to netem 'rate' (e.g., \"10mbit\", \"10mbit 1000 500\")."`
	Slot          string `json:"slot,omitempty" jsonschema:"Pass-through to netem 'slot' (e.g., \"5ms 10ms packets 64\")."`
	Child         string `json:"child,omitempty" jsonschema:"Child qdisc kind attached at parent 1: (one of: cake, fq_codel, pie, sfq, codel; see npte's validate.AllowedChildQdiscs for the authoritative list)."`
	CakeBandwidth string `json:"cakeBandwidth,omitempty" jsonschema:"Bandwidth for the 'cake' child qdisc (e.g., \"30mbit\"). Ignored unless child == \"cake\"."`
}

// NetemApply is the MCP handler for the `netem_apply` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) NetemApply(ctx context.Context, req *mcp.CallToolRequest,
	input *netemApplyInput) (*mcp.CallToolResult, *runOutput, error) {
	if err := validateNetemTarget(input.Netns, input.Iface); err != nil {
		return nil, nil, err
	}
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
	out, err := sm.runProc(ctx, defaultWaitTimeout, args)
	return nil, out, err
}

// netemClearInput is the input schema for the `netem_clear` MCP tool.
type netemClearInput struct {
	Netns string `json:"netns" jsonschema:"Must be \"router\". The MCP only shapes at the canonical hub of the canned lab; shaping at a leaf namespace (client, server) is rejected. See server instructions for the lab shaping convention."`
	Iface string `json:"iface" jsonschema:"Must be \"if-client\" or \"if-server\" — the two router-side veth endpoints of the canned lab."`
}

// NetemClear is the MCP handler for the `netem_clear` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) NetemClear(ctx context.Context, req *mcp.CallToolRequest,
	input *netemClearInput) (*mcp.CallToolResult, *runOutput, error) {
	if err := validateNetemTarget(input.Netns, input.Iface); err != nil {
		return nil, nil, err
	}
	out, err := sm.runProc(ctx, defaultWaitTimeout,
		[]string{"netem", "clear", "--", input.Netns, input.Iface})
	return nil, out, err
}
