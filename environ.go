// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"io"
	"os"
	"os/exec"

	"github.com/bassosimone/runtimex"
	"github.com/rogpeppe/go-internal/lockedfile"
)

// environ abstracts away side effects (filesystem, execution, locking, exit)
// so that commands can be tested without root, namespaces, or real I/O.
type environ struct {
	Exit             func(code int)
	Stdout           io.Writer
	Stderr           io.Writer
	MkdirAll         func(path string, perm os.FileMode) error
	ReadFile         func(name string) ([]byte, error)
	WriteFile        func(name string, data []byte, perm os.FileMode) error
	Stat             func(name string) (os.FileInfo, error)
	Remove           func(name string) error
	RunCommand       func(cmd *exec.Cmd) error
	LookPath         func(file string) (string, error)
	LockFile         func(path string) (func(), error)
	LogFatalOnError0 func(err error)
}

// newEnvironOS returns an environ wired to real OS operations.
func newEnvironOS() *environ {
	return &environ{
		Exit:       os.Exit,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		MkdirAll:   os.MkdirAll,
		ReadFile:   os.ReadFile,
		WriteFile:  os.WriteFile,
		Stat:       os.Stat,
		Remove:     os.Remove,
		RunCommand: func(cmd *exec.Cmd) error { return cmd.Run() },
		LookPath:   exec.LookPath,
		LockFile: func(path string) (func(), error) {
			return lockedfile.MutexAt(path).Lock()
		},
		LogFatalOnError0: runtimex.LogFatalOnError0,
	}
}

// env is the global environ used by all commands.
var env = newEnvironOS()
