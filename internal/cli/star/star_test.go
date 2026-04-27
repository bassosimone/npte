// SPDX-License-Identifier: GPL-3.0-or-later

package star

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/cli/clitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain_Dispatcher walks the declarative wiring in Main; the leaves
// have their own tables.
func TestMain_Dispatcher(t *testing.T) {
	s := clitest.Setup(t)
	require.NoError(t, Main(context.Background(), []string{"destroy", "--dry-run"}))
	assert.Equal(t, -1, s.ExitCode)
	assert.Len(t, s.Commands, 3)
}
