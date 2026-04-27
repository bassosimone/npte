// SPDX-License-Identifier: GPL-3.0-or-later

package container

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain_Dispatcher exercises the declarative wiring in Main by invoking
// it with a non-panicking subcommand+args combo. The leaf is already
// covered by its own table; this is just to walk the dispatcher lines.
func TestMain_Dispatcher(t *testing.T) {
	s := testenv.Setup(t)
	require.NoError(t, Main(context.Background(),
		[]string{"create", "--dry-run", "noble", "/var/lib/machines/test"}))
	assert.Equal(t, -1, s.ExitCode)
}
