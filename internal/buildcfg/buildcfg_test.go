// SPDX-License-Identifier: GPL-3.0-or-later

package buildcfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveVersion(t *testing.T) {
	t.Run("ldflags override wins", func(t *testing.T) {
		assert.Equal(t, "v1.2.3", resolveVersion("v1.2.3", "v0.0.1"))
	})

	t.Run("module version used when no ldflags", func(t *testing.T) {
		assert.Equal(t, "v0.9.0", resolveVersion("", "v0.9.0"))
	})

	t.Run("fallback to devel", func(t *testing.T) {
		assert.Equal(t, "(devel)", resolveVersion("", ""))
	})
}

func TestInitSetsVersion(t *testing.T) {
	assert.NotEmpty(t, Version)
}

func TestInstallPathDefault(t *testing.T) {
	assert.Equal(t, "/usr/local/sbin/npte", InstallPath)
}
