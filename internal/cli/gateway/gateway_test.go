// SPDX-License-Identifier: GPL-3.0-or-later

package gateway

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain_Dispatcher exercises the declarative wiring in Main; see the
// container package's equivalent test for the rationale.
func TestMain_Dispatcher(t *testing.T) {
	s := testenv.Setup(t)
	require.NoError(t, Main(context.Background(),
		[]string{"destroy", "--dry-run", "router"}))
	assert.Equal(t, -1, s.ExitCode)
}
