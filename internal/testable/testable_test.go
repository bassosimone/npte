// SPDX-License-Identifier: GPL-3.0-or-later

package testable

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bassosimone/deferexit"
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

func TestNewEnvironOS_StartCommand(t *testing.T) {
	env := NewEnvironOS()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	require.NoError(t, env.StartCommand(cmd))
	require.NoError(t, cmd.Wait())
}

func TestNewEnvironOS_WaitCommand(t *testing.T) {
	env := NewEnvironOS()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	require.NoError(t, cmd.Start())
	require.NoError(t, env.WaitCommand(cmd))
}

func TestNewEnvironOS_ProcessSignal(t *testing.T) {
	env := NewEnvironOS()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	require.NoError(t, cmd.Start())
	require.NoError(t, env.ProcessSignal(cmd, os.Interrupt))
	_ = cmd.Wait()
}

// TestNewEnvironOS_LogFatalOnError0_nilIsNoOp covers the fast path:
// passing nil must do nothing (no log, no panic, no exit).
func TestNewEnvironOS_LogFatalOnError0_nilIsNoOp(t *testing.T) {
	env := NewEnvironOS()
	assert.NotPanics(t, func() {
		env.LogFatalOnError0(nil)
	})
}

// TestNewEnvironOS_LogFatalOnError0_errorLogsAndExits covers the failure
// path: a non-nil error is written to the log package's default writer
// and then deferexit.Panic(1) fires. We redirect log output to a buffer
// and use deferexit.Run to recover the synthetic exit code.
func TestNewEnvironOS_LogFatalOnError0_errorLogsAndExits(t *testing.T) {
	env := NewEnvironOS()

	var buf bytes.Buffer
	origOut := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0) // suppress the timestamp prefix so the assertion is stable
	t.Cleanup(func() {
		log.SetOutput(origOut)
		log.SetFlags(origFlags)
	})

	code := deferexit.Run(func() {
		env.LogFatalOnError0(errors.New("boom"))
	})

	assert.Equal(t, 1, code)
	assert.Equal(t, "boom\n", buf.String())
}
