// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// labCreateInput is the (empty) input schema for the `lab_create`
// MCP tool. The lab topology is fully canned.
type labCreateInput struct{}

// defaultWaitTimeout is the default timeout used for running processes
// that should complete in a few seconds and should not block. We use
// a large enough timeout that, if we hit the timeout, it means that
// something has really went wrong with the process run.
const defaultWaitTimeout = 30 * time.Second

// LabCreate is the MCP handler for the `lab_create` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) LabCreate(ctx context.Context, req *mcp.CallToolRequest,
	input *labCreateInput) (*mcp.CallToolResult, *runOutput, error) {
	out, err := sm.runProc(ctx, defaultWaitTimeout, []string{"lab", "create"})
	return nil, out, err
}

// labDestroyInput is the (empty) input schema for the `lab_destroy`
// MCP tool. The lab topology is fully canned.
type labDestroyInput struct{}

// LabDestroy is the MCP handler for the `lab_destroy` tool. See the
// tool's registration in [serveMain] for the agent-facing description.
func (sm *sessionManager) LabDestroy(ctx context.Context, req *mcp.CallToolRequest,
	input *labDestroyInput) (*mcp.CallToolResult, *runOutput, error) {
	out, err := sm.runProc(ctx, defaultWaitTimeout, []string{"lab", "destroy"})
	return nil, out, err
}
