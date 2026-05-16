// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServeMain_EmptyStdin exercises the full startup→shutdown path of
// serveMain by feeding an empty reader as stdin. The MCP transport hits
// EOF immediately, causing server.Run to return and the cleanup path
// to execute.
func TestServeMain_EmptyStdin(t *testing.T) {
	s := testenv.Setup(t)
	testable.Env.Stdin = strings.NewReader("")
	require.NoError(t, serveMain(context.Background(), nil))
	assert.Equal(t, -1, s.ExitCode)
}
