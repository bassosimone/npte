// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"testing"

	"github.com/bassosimone/npte/internal/cli/clitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	// list does not support --dry-run; with the empty ReadDir stub from
	// clitest.Setup it just produces empty stdout.
	s := clitest.Setup(t)
	require.NoError(t, listMain(context.Background(), nil))
	assert.Equal(t, -1, s.ExitCode)
	assert.Equal(t, "", s.Stdout.String())
}
