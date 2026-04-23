// SPDX-License-Identifier: GPL-3.0-or-later

package subprocess

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/stretchr/testify/assert"
)

// Make sure that MustRun actually runs a command. We re-invoke the test
// binary itself with a regex that matches no test, which exits 0 on every
// OS and keeps the test portable.
func TestMustRun(t *testing.T) {
	var captured error
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = io.Discard
	env.Stderr = io.Discard
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	MustRun(context.Background(), false, os.Args[0], "-test.run=^$")
	assert.NoError(t, captured)
}

// Make sure that MustRun in dry mode prints the command to stdout
// rather than executing it.
func TestMustRun_Dry(t *testing.T) {
	var captured error
	var stdout bytes.Buffer
	orig := testable.Env
	env := testable.NewEnvironOS()
	env.Stdout = &stdout
	env.Stderr = io.Discard
	env.LogFatalOnError0 = func(err error) { captured = err }
	testable.Env = env
	t.Cleanup(func() { testable.Env = orig })

	MustRun(context.Background(), true, "echo", "hello world")
	assert.NoError(t, captured)
	assert.Equal(t, "echo 'hello world'\n", stdout.String())
}
