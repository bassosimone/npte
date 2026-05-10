// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// labCreateInput is the (empty) input schema for the `start_lab_create`
// MCP tool. The lab topology is fully canned.
type labCreateInput struct{}

// LabCreate is the MCP handler for the `start_lab_create` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) LabCreate(ctx context.Context, req *mcp.CallToolRequest,
	input *labCreateInput) (*mcp.CallToolResult, *startOutput, error) {
	out, err := sm.startProc([]string{"lab", "create"})
	return nil, out, err
}

// labDestroyInput is the (empty) input schema for the `start_lab_destroy`
// MCP tool. The lab topology is fully canned.
type labDestroyInput struct{}

// LabDestroy is the MCP handler for the `start_lab_destroy` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) LabDestroy(ctx context.Context, req *mcp.CallToolRequest,
	input *labDestroyInput) (*mcp.CallToolResult, *startOutput, error) {
	out, err := sm.startProc([]string{"lab", "destroy"})
	return nil, out, err
}
