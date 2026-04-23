// SPDX-License-Identifier: GPL-3.0-or-later

package testable

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Make sure that fields are all populated to nonzero values.
func TestNewEnvironOS_AllFieldsPopulated(t *testing.T) {
	env := NewEnvironOS()
	value := reflect.ValueOf(*env)
	for idx := 0; idx < value.NumField(); idx++ {
		field := value.Field(idx)
		name := value.Type().Field(idx).Name
		assert.Falsef(t, field.IsZero(), "field %s is zero", name)
	}
}

// Make sure that LockFile actually locks a file.
func TestNewEnvironOS_LockFile(t *testing.T) {
	env := NewEnvironOS()
	path := filepath.Join(t.TempDir(), "test.lock")
	unlock, err := env.LockFile(path)
	require.NoError(t, err)
	unlock()
}

// Make sure that RunCommand actually runs a command. We re-invoke the
// test binary itself with a regex that matches no test, which exits 0
// on every OS and keeps the test portable.
func TestNewEnvironOS_RunCommand(t *testing.T) {
	env := NewEnvironOS()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	require.NoError(t, env.RunCommand(cmd))
}
