// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// shapeInput is the input schema for the `shape_download` and
// `shape_upload` MCP tools.
//
// The target namespace and interface are implicit in the tool name:
// shape_download operates on router:if-client (router->client egress);
// shape_upload operates on router:if-server (router->server egress).
type shapeInput struct {
	Delay         string `json:"delay,omitempty" jsonschema:"Pass-through to netem 'delay' (e.g., \"10ms\", \"10ms 2ms distribution paretonormal\")."`
	Loss          string `json:"loss,omitempty" jsonschema:"Pass-through to netem 'loss' (e.g., \"1%\", \"gemodel 0.1 0.05 0.9 0.95\")."`
	Limit         string `json:"limit,omitempty" jsonschema:"Pass-through to netem 'limit' in packets (e.g., \"1000\")."`
	Rate          string `json:"rate,omitempty" jsonschema:"Pass-through to netem 'rate' (e.g., \"10mbit\", \"10mbit 1000 500\")."`
	Slot          string `json:"slot,omitempty" jsonschema:"Pass-through to netem 'slot' (e.g., \"5ms 10ms packets 64\")."`
	Child         string `json:"child,omitempty" jsonschema:"Child qdisc kind attached at parent 1: (one of: cake, fq_codel, pie, sfq, codel; see npte's validate.AllowedChildQdiscs for the authoritative list)."`
	CakeBandwidth string `json:"cakeBandwidth,omitempty" jsonschema:"Bandwidth for the 'cake' child qdisc (e.g., \"30mbit\"). Ignored unless child == \"cake\"."`
}

// shapeApply clears any existing qdisc then applies a fresh netem
// configuration on the given (netns, iface) pair.
func (sm *sessionManager) shapeApply(
	ctx context.Context, netns, iface string, input *shapeInput) (*multiRunOutput, error) {
	clearResult, err := sm.runProc(ctx, defaultWaitTimeout,
		[]string{"netem", "clear", "--", netns, iface})
	if err != nil {
		return nil, err
	}

	args := []string{"netem", "apply"}
	for _, kv := range []struct{ key, value string }{
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
	args = append(args, "--", netns, iface)

	applyResult, err := sm.runProc(ctx, defaultWaitTimeout, args)
	if err != nil {
		return nil, err
	}

	return &multiRunOutput{
		Steps: []*runStep{
			clearResult.toStep("clear"),
			applyResult.toStep("apply"),
		},
	}, nil
}

// ShapeDownload is the MCP handler for the `shape_download` tool. See
// the tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) ShapeDownload(ctx context.Context, req *mcp.CallToolRequest,
	input *shapeInput) (*mcp.CallToolResult, *multiRunOutput, error) {
	out, err := sm.shapeApply(ctx, "router", "if-client", input)
	return nil, out, err
}

// ShapeUpload is the MCP handler for the `shape_upload` tool. See
// the tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) ShapeUpload(ctx context.Context, req *mcp.CallToolRequest,
	input *shapeInput) (*mcp.CallToolResult, *multiRunOutput, error) {
	out, err := sm.shapeApply(ctx, "router", "if-server", input)
	return nil, out, err
}

// shapeClearInput is the (empty) input schema for the `shape_clear` MCP tool.
type shapeClearInput struct{}

// ShapeClear is the MCP handler for the `shape_clear` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) ShapeClear(ctx context.Context, req *mcp.CallToolRequest,
	input *shapeClearInput) (*mcp.CallToolResult, *multiRunOutput, error) {
	clearDown, err := sm.runProc(ctx, defaultWaitTimeout,
		[]string{"netem", "clear", "--", "router", "if-client"})
	if err != nil {
		return nil, nil, err
	}
	clearUp, err := sm.runProc(ctx, defaultWaitTimeout,
		[]string{"netem", "clear", "--", "router", "if-server"})
	if err != nil {
		return nil, nil, err
	}
	return nil, &multiRunOutput{
		Steps: []*runStep{
			clearDown.toStep("clear download"),
			clearUp.toStep("clear upload"),
		},
	}, nil
}
